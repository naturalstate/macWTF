package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/naturalstate/macWTF/internal/install"
	"github.com/naturalstate/macWTF/internal/manifest"
)

var (
	barFill   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#A78BFA"})
	barEmpty  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#374151"})
	okMark    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#047857", Dark: "#34D399"}).Bold(true)
	failMark  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"}).Bold(true)
	dimText   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#6B7280"})
	boldText  = lipgloss.NewStyle().Bold(true)
	warnText  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"})
	spinChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
)

// runner renders install progress to a terminal.
//
// The design constraint: brew is verbose, and a 60-tool run would scroll
// thousands of lines past the user. So command output is shown as a single
// transient line that is overwritten, while completed tools scroll normally as
// a permanent record. The bar stays pinned at the bottom.
type runner struct {
	total     int
	done      int
	current   string
	lastLine  string
	spin      int
	startedAt time.Time
	tty       bool
	width     int

	// recent holds the tail of the current tool's output. Progress output
	// is transient by design, which means a failing command's error would
	// be overwritten and lost — the one moment the user most needs to see
	// it. Keeping a small ring lets the failure be replayed.
	recent []string
}

// recentLimit is how many trailing output lines to keep for a failure report.
// Enough to carry a package manager's error and its context, not so many that
// a failing tool floods the screen.
const recentLimit = 12

func newRunner(total int) *runner {
	return &runner{
		total:     total,
		startedAt: time.Now(),
		tty:       isTerminal(os.Stdout),
		width:     terminalWidth(),
	}
}

func (r *runner) handle(ev install.Event) {
	switch ev.Kind {
	case install.EventRunStart:
		fmt.Println()

	case install.EventToolStart:
		r.current = ev.Tool.ID
		r.recent = r.recent[:0]
		r.render()

	case install.EventStepStart:
		r.spin++
		r.setStatus(ev.Step.Desc)

	case install.EventOutput:
		r.spin++
		// Homebrew's own progress lines are noise here; the bar already
		// says what is happening. Only surface lines that matter.
		line := strings.TrimSpace(ev.Line)
		if line == "" {
			return
		}
		r.recent = append(r.recent, line)
		if len(r.recent) > recentLimit {
			r.recent = r.recent[len(r.recent)-recentLimit:]
		}
		r.setStatus(line)

	case install.EventToolDone:
		r.clearLine()
		if ev.Err != nil {
			fmt.Printf("  %s %-16s %s\n", failMark.Render("✗"), ev.Tool.ID,
				dimText.Render(ev.Err.Error()))
			// Replay what the command actually said. An exit status
			// on its own is not a diagnosis.
			for _, line := range r.recent {
				fmt.Printf("      %s\n", dimText.Render(truncateLine(line, r.width-8)))
			}
			if len(r.recent) > 0 {
				fmt.Println()
			}
		} else {
			fmt.Printf("  %s %-16s %s\n", okMark.Render("✓"), ev.Tool.ID,
				dimText.Render(fmt.Sprintf("%.1fs", ev.Elapsed.Seconds())))
		}
		r.done++
		r.render()

	case install.EventRunDone:
		r.clearLine()
	}
}

// setStatus updates the transient status line under the bar.
func (r *runner) setStatus(text string) {
	r.lastLine = text
	r.render()
}

func (r *runner) render() {
	if !r.tty {
		return
	}
	r.clearLine()

	const barWidth = 28
	filled := 0
	if r.total > 0 {
		filled = r.done * barWidth / r.total
	}
	if filled > barWidth {
		filled = barWidth
	}
	styled := barFill.Render(strings.Repeat("█", filled)) +
		barEmpty.Render(strings.Repeat("░", barWidth-filled))

	spinner := spinChars[r.spin%len(spinChars)]
	head := fmt.Sprintf("  %s %s %d/%d", spinner, styled, r.done, r.total)

	tail := r.current
	if r.lastLine != "" {
		tail = r.current + " · " + r.lastLine
	}

	// Keep the whole thing on one line so it can be overwritten cleanly.
	avail := r.width - lipgloss.Width(head) - 4
	if avail < 10 {
		avail = 10
	}
	if lipgloss.Width(tail) > avail {
		tr := []rune(tail)
		if len(tr) > avail-1 {
			tr = tr[:avail-1]
		}
		tail = string(tr) + "…"
	}

	fmt.Printf("%s  %s", head, dimText.Render(tail))
}

func (r *runner) clearLine() {
	if !r.tty {
		return
	}
	fmt.Printf("\r\033[2K")
}

func (r *runner) finish() {
	r.clearLine()
	if r.tty {
		fmt.Println()
	}
}

// stdinReader is shared across every prompt on purpose. A bufio.Reader reads
// ahead, so constructing a new one per question throws away whatever the
// previous one buffered — with two prompts in a row the second silently sees
// EOF and answers "no" no matter what the user typed.
var stdinReader = bufio.NewReader(os.Stdin)

// confirm asks a yes/no question. Defaults to no: anything that modifies the
// machine should require a deliberate yes, never an accidental RETURN.
func confirm(question string) bool {
	fmt.Printf("%s [y/N] ", question)
	line, err := stdinReader.ReadString('\n')
	if err != nil {
		fmt.Println()
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

// confirmQuarantine asks specifically about stripping Gatekeeper quarantine.
// Separate from the install confirmation on purpose: agreeing to install a tool
// is not agreeing to waive a malware check on it.
func confirmQuarantine(tools []*manifest.Tool, assumeYes bool) bool {
	fmt.Println()
	fmt.Println(warnText.Render(fmt.Sprintf(
		"%d tool(s) are unsigned and will not launch until macOS's", len(tools))))
	fmt.Println(warnText.Render("quarantine attribute is removed:"))
	fmt.Println()
	for _, t := range tools {
		fmt.Printf("  %-14s %s\n", t.ID, t.AppPath)
		fmt.Printf("  %s\n", dimText.Render("xattr -d -r com.apple.quarantine "+t.AppPath))
	}
	fmt.Println()
	fmt.Println("Removing it bypasses Gatekeeper's malware check for these specific")
	fmt.Println("apps. Declining is safe: they install either way, and you can clear")
	fmt.Println("quarantine later by right-clicking the app and choosing Open.")
	fmt.Println()

	if assumeYes {
		fmt.Println(boldText.Render("--yes given: stripping quarantine."))
		return true
	}
	return confirm("Remove quarantine from these apps?")
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// terminalWidth returns the terminal width, defaulting to something sane when
// it cannot be determined.
func terminalWidth() int {
	if w, _, err := termSize(); err == nil && w > 0 {
		return w
	}
	return 80
}

// truncateLine shortens a line of command output to fit the terminal.
func truncateLine(s string, w int) string {
	if w < 20 {
		w = 20
	}
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	return string(r[:w-1]) + "…"
}
