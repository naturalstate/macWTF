package backend

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/naturalstate/macWTF/internal/manifest"
)

// Curl installs from a direct download.
//
// A meaningful share of security tooling is published only as a GitHub release
// asset — SDR++, SigDigger, AzureHound, the SharpHound collectors — with no
// package manager carrying it at all. Without this backend those entries are
// dead weight in the catalogue.
//
// The shape of the download decides how it is handled: a bare binary is made
// executable and moved into place, archives are unpacked, and a disk image is
// mounted so the application inside can be copied out.
type Curl struct{}

func (c *Curl) ID() manifest.Backend { return manifest.BackendCurl }

// Preflight has nothing to check: curl, tar, unzip and hdiutil all ship with
// macOS.
func (c *Curl) Preflight(ctx *Ctx) error { return nil }

// binDir is where downloaded executables land. Under the user's home rather
// than /usr/local, so nothing here needs root.
func (c *Curl) binDir(ctx *Ctx) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp"
	}
	return filepath.Join(home, ".local", "bin")
}

// Installed lists what has already been downloaded, by binary name and by
// application bundle.
func (c *Curl) Installed(ctx *Ctx) (map[string]bool, error) {
	set := map[string]bool{}

	if entries, err := os.ReadDir(c.binDir(ctx)); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				set[e.Name()] = true
			}
		}
	}
	if entries, err := os.ReadDir("/Applications"); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".app") {
				set[e.Name()] = true
				set[strings.TrimSuffix(e.Name(), ".app")] = true
			}
		}
	}
	return set, nil
}

// InstalledKey is the artefact the download produces: an explicit binary name,
// the application bundle, or the last path element of the URL.
func (c *Curl) InstalledKey(t *manifest.Tool) string {
	if t.Binary != "" {
		return t.Binary
	}
	if t.AppPath != "" {
		return filepath.Base(t.AppPath)
	}
	return downloadName(t.Package)
}

// downloadName is the filename a URL would save as.
func downloadName(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Path != "" {
		return path.Base(u.Path)
	}
	return path.Base(raw)
}

func (c *Curl) InstallPlan(t *manifest.Tool, ctx *Ctx) ([]Step, error) {
	if !strings.HasPrefix(t.Package, "http://") && !strings.HasPrefix(t.Package, "https://") {
		return nil, fmt.Errorf("curl backend needs a URL, got %q", t.Package)
	}

	bin := c.binDir(ctx)
	tmp := filepath.Join(os.TempDir(), "macwtf-dl", t.ID)
	file := downloadName(t.Package)
	dl := filepath.Join(tmp, file)

	steps := []Step{{
		Desc:  "download " + file,
		Args:  []string{fmt.Sprintf("mkdir -p %q && curl -fSL --retry 2 -o %q %q", tmp, dl, t.Package)},
		Kind:  KindInstall,
		Shell: true,
	}}

	name := t.Binary
	if name == "" {
		name = t.ID
	}

	switch {
	case strings.HasSuffix(file, ".dmg"):
		// Mount read-only into a private point, copy the application
		// out, and always detach — a leaked mount is worse than a
		// failed install because it persists after the run.
		app := t.AppPath
		if app == "" {
			return nil, fmt.Errorf("a .dmg download needs app_path so macWTF knows what to copy out")
		}
		steps = append(steps, Step{
			Desc: "mount and copy " + filepath.Base(app),
			Args: []string{fmt.Sprintf(
				`mnt=$(mktemp -d) && hdiutil attach -nobrowse -readonly -mountpoint "$mnt" %q >/dev/null && `+
					`cp -R "$mnt"/*.app %q ; rc=$? ; hdiutil detach "$mnt" >/dev/null 2>&1 ; exit $rc`,
				dl, filepath.Dir(app))},
			Kind:  KindInstall,
			Shell: true,
		})

	case strings.HasSuffix(file, ".zip"):
		steps = append(steps, Step{
			Desc:  "unpack " + file,
			Args:  []string{fmt.Sprintf(`cd %q && unzip -oq %q`, tmp, dl)},
			Kind:  KindInstall,
			Shell: true,
		})
		steps = append(steps, placeArtifact(t, tmp, bin, name))

	case strings.HasSuffix(file, ".tar.gz"), strings.HasSuffix(file, ".tgz"),
		strings.HasSuffix(file, ".tar.xz"), strings.HasSuffix(file, ".tar.bz2"):
		steps = append(steps, Step{
			Desc:  "unpack " + file,
			Args:  []string{fmt.Sprintf(`cd %q && tar xf %q`, tmp, dl)},
			Kind:  KindInstall,
			Shell: true,
		})
		steps = append(steps, placeArtifact(t, tmp, bin, name))

	case strings.HasSuffix(file, ".pkg"):
		// Installer packages write wherever their payload says, which
		// is usually outside the home directory, so this one needs
		// elevation and says so.
		steps = append(steps, Step{
			Desc: "run installer " + file,
			Name: "installer",
			Args: []string{"-pkg", dl, "-target", "/"},
			Kind: KindInstall,
			Sudo: true,
		})

	default:
		// A bare binary.
		steps = append(steps, Step{
			Desc:  "install " + name,
			Args:  []string{fmt.Sprintf(`mkdir -p %q && chmod +x %q && mv %q %q`, bin, dl, dl, filepath.Join(bin, name))},
			Kind:  KindInstall,
			Shell: true,
		})
	}

	return commonSteps(steps, t, ctx), nil
}

// placeArtifact moves whatever an archive unpacked to into place: an
// application bundle into /Applications, otherwise the named binary into the
// user's bin directory.
func placeArtifact(t *manifest.Tool, tmp, bin, name string) Step {
	if t.AppPath != "" {
		return Step{
			Desc: "install " + filepath.Base(t.AppPath),
			Args: []string{fmt.Sprintf(
				`app=$(find %q -maxdepth 3 -name '*.app' -print -quit) && `+
					`[ -n "$app" ] && rm -rf %q && cp -R "$app" %q`,
				tmp, t.AppPath, t.AppPath)},
			Kind:  KindInstall,
			Shell: true,
		}
	}
	return Step{
		Desc: "install " + name,
		Args: []string{fmt.Sprintf(
			`mkdir -p %q && f=$(find %q -type f -name %q -perm -u+x -print -quit) && `+
				`[ -n "$f" ] && mv "$f" %q && chmod +x %q`,
			bin, tmp, name, filepath.Join(bin, name), filepath.Join(bin, name))},
		Kind:  KindInstall,
		Shell: true,
	}
}

func (c *Curl) RemovePlan(t *manifest.Tool, ctx *Ctx) ([]Step, error) {
	target := filepath.Join(c.binDir(ctx), c.InstalledKey(t))
	if t.AppPath != "" {
		target = t.AppPath
	}
	return []Step{{
		Desc:  "remove " + target,
		Args:  []string{fmt.Sprintf("rm -rf %q", target)},
		Kind:  KindRemove,
		Shell: true,
	}}, nil
}
