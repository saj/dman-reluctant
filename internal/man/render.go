package man

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/google/shlex"
	"github.com/mattn/go-isatty"
	"github.com/mattn/go-tty"
)

const (
	defaultColumns = 78
	maxColumns     = 150
)

var defaultPager = []string{"less", "-is"} // man(1) default

func Render(r io.Reader) error {
	fmtErr := func(err error) error {
		return fmt.Errorf("render: %s", err)
	}

	f, err := typeset(r)
	if err != nil {
		return fmtErr(err)
	}
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

	if err := output(f); err != nil {
		return fmtErr(err)
	}
	return nil
}

func typeset(r io.Reader) (*os.File, error) {
	fmtErr := func(err error) error {
		return fmt.Errorf("typeset: %s", err)
	}

	f, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, fmtErr(err)
	}

	lineLength := fmt.Sprintf("-rLL=%dn", columns())

	// In utf8 mode, grotty will output Unicode HYPHEN codepoints for --option
	// lists in man pages.  Meanwhile, striking the dash key on your keyboard
	// will likely produce the technically distinct, but visually identical,
	// HYPHEN-MINUS codepoint.  This misalignment can make it difficult to
	// search for options in the pager.  'ascii' is the crude hammer that forces
	// these dashes to align.
	groff := exec.Command("groff", "-T", "ascii", "-m", "mandoc", lineLength)
	// grotty is documented to enable SGR mode by default.  (In SGR mode, grotty
	// will emit ANSI escape sequences in order to represent boldfaced and
	// underlined text.)  Contrary to the documentation, however, the grotty
	// implementations on some platforms -- namely macOS -- do not seem to
	// enable SGR mode by default.  This can lead to inconsistent output from
	// dman.  Users on macOS will see overstriking while users on other
	// platforms will see ANSI escape sequences.  SGR mode is explicitly
	// disabled here, to force overstriking, in order to improve consistency.
	// It is easier to explicitly disable SGR than it is to explicitly enable.
	groff.Env = append(os.Environ(), "GROFF_NO_SGR=")
	groff.Stdin = r
	groff.Stdout = f
	groff.Stderr = os.Stderr
	if err := groff.Run(); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, fmtErr(err)
	}

	_, err = f.Seek(0, os.SEEK_SET)
	if err != nil {
		f.Close()
		return nil, fmtErr(err)
	}
	return f, nil
}

func output(f *os.File) error {
	fn := outputDump
	if isatty.IsTerminal(os.Stdout.Fd()) {
		fn = outputPager
	}
	if err := fn(f); err != nil {
		return fmt.Errorf("output: %s", err)
	}
	return nil
}

func outputPager(f *os.File) error {
	argv := preferredPager()
	argv = append(argv, f.Name())
	pager := exec.Command(argv[0], argv[1:]...)
	pager.Stdout = os.Stdout
	pager.Stderr = os.Stderr
	return pager.Run()
}

func outputDump(f *os.File) error {
	_, err := io.Copy(os.Stdout, f)
	return err
}

func columns() int {
	tty, err := tty.Open()
	if err != nil {
		return defaultColumns
	}
	defer tty.Close()

	w, _, err := tty.Size()
	if err != nil {
		return defaultColumns
	}

	if w > maxColumns {
		return maxColumns
	}
	return w
}

func preferredPager() []string {
	if argv, ok := shlexEnv("MANPAGER"); ok {
		return argv
	}
	if argv, ok := shlexEnv("PAGER"); ok {
		return argv
	}
	return defaultPager
}

func shlexEnv(key string) ([]string, bool) {
	val := os.Getenv(key)
	if val == "" {
		return nil, false
	}
	argv, err := shlex.Split(val)
	if err != nil {
		return nil, false
	}
	return argv, true
}
