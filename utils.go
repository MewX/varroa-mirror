package varroa

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"regexp"

	"github.com/subosito/norma"
	"github.com/ttacon/chalk"
)

//-----------------------------------------------------------------------------

func startOfDay(t time.Time) time.Time {
	return t.Truncate(24 * time.Hour)
}

func previousDay(t time.Time) time.Time {
	return t.Add(time.Duration(-24) * time.Hour)
}

func nextDay(t time.Time) time.Time {
	return t.Add(time.Duration(24) * time.Hour)
}

func allDaysSince(t time.Time) []time.Time {
	firstDay := startOfDay(t)
	tomorrow := nextDay(startOfDay(time.Now()))
	dayTimes := []time.Time{}
	for t := firstDay; t.Before(tomorrow); t = nextDay(t) {
		dayTimes = append(dayTimes, t)
	}
	return dayTimes
}

//-----------------------------------------------------------------------------

// StringInSlice checks if a string is in a []string, returns bool.
func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// MatchInSlice checks if a string regexp-matches a slice of patterns, returns bool
func MatchInSlice(a string, b []string) bool {
	for _, pattern := range b {
		if strings.HasPrefix(pattern, "r:") {
			pattern = strings.Replace(pattern, "r:", "", 1)
			// try to match
			match, err := regexp.MatchString(pattern, a)
			if err != nil {
				logThis.Error(err, VERBOSE)
			}
			if match {
				return true
			}
		} else if pattern == a {
			return true
		}
	}
	return false
}

// IntInSlice checks if an int is in a []int, returns bool.
func IntInSlice(a int, list []int) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func IntSliceToString(in []int) string {
	b := make([]string, len(in))
	for i, v := range in {
		b[i] = strconv.Itoa(v)
	}
	return strings.Join(b, " ")
}

func StringSliceToIntSlice(in []string) ([]int, error) {
	var err error
	b := make([]int, len(in))
	for i, v := range in {
		b[i], err = strconv.Atoi(v)
		if err != nil {
			return []int{}, err
		}
	}
	return b, nil
}

func IntSliceToStringSlice(in []int) []string {
	b := make([]string, len(in))
	for i, v := range in {
		b[i] = strconv.Itoa(v)
	}
	return b
}

func CommonInStringSlices(X, Y []string) []string {
	m := make(map[string]bool)
	for _, y := range Y {
		m[y] = true
	}
	var ret []string
	for _, x := range X {
		if m[x] {
			ret = append(ret, x)
		}
	}
	return ret
}

func RemoveFromSlice(r string, s []string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

func RemoveStringSliceDuplicates(elements []string) []string {
	// Use map to record duplicates as we find them.
	encountered := map[string]bool{}
	result := []string{}

	for v := range elements {
		if encountered[elements[v]] == true {
			// Do not add duplicate.
		} else {
			// Record this element as an encountered element.
			encountered[elements[v]] = true
			// Append to result slice.
			result = append(result, elements[v])
		}
	}
	// Return the new slice.
	return result
}

func checkErrors(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

//-----------------------------------------------------------------------------

func SanitizeFolder(path string) string {
	// making sure the path is relative
	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}

	// TODO check it's not more than 250 characters long!

	// making sure the final filename is valid
	return norma.Sanitize(path)
}

// DirectoryExists checks if a directory exists.
func DirectoryExists(path string) (res bool) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if info.IsDir() {
		return true
	}
	return
}

// AbsoluteFileExists checks if an absolute path is an existing file.
func AbsoluteFileExists(path string) (res bool) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if info.Mode().IsRegular() {
		return true
	}
	return
}

// FileExists checks if a path is valid
func FileExists(path string) bool {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	return AbsoluteFileExists(absolutePath)
}

// CopyFile copies a file from src to dst. If src and dst files exist, and are
// the same, then return success. Copy the file contents from src to dst.
func CopyFile(src, dst string, useHardLinks bool) (err error) {
	sfi, err := os.Stat(src)
	if err != nil {
		return
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return fmt.Errorf("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
	}
	dfi, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return fmt.Errorf("CopyFile: non-regular destination file %s (%q)", dfi.Name(), dfi.Mode().String())
		}
		if os.SameFile(sfi, dfi) {
			return
		}
	}
	if useHardLinks {
		return os.Link(src, dst)
	}
	return copyFileContents(src, dst)
}

// copyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

// CopyDir recursively copies a directory tree, attempting to preserve permissions.
// Source directory must exist, destination directory must *not* exist.
// Symlinks are ignored and skipped.
func CopyDir(src, dst string, useHardLinks bool) (err error) {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !si.IsDir() {
		return errors.New("Source is not a directory")
	}
	_, err = os.Stat(dst)
	if err != nil && !os.IsNotExist(err) {
		return
	}
	if err == nil {
		return errors.New("Destination already exists")
	}
	err = os.MkdirAll(dst, si.Mode())
	if err != nil {
		return
	}
	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			err = CopyDir(srcPath, dstPath, useHardLinks)
			if err != nil {
				return
			}
		} else {
			// Skip symlinks.
			if entry.Mode()&os.ModeSymlink != 0 {
				continue
			}
			err = CopyFile(srcPath, dstPath, useHardLinks)
			if err != nil {
				return
			}
		}
	}
	return
}

// DirectoryIsEmpty checks if a directory is empty.
func DirectoryIsEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	// not empty or error
	return false, err
}

// DirectoryContainsMusic returns true if it contains mp3 or flac files.
func DirectoryContainsMusic(directoryPath string) bool {
	if err := filepath.Walk(directoryPath, func(path string, f os.FileInfo, err error) error {
		if StringInSlice(strings.ToLower(filepath.Ext(path)), []string{mp3Ext, flacExt}) {
			// stop walking the directory as soon as a track is found
			return errors.New(foundMusic)
		}
		return nil
	}); err == nil || err.Error() != foundMusic {
		return false
	}
	return true
}

// TimeTrack helps track the time taken by a function.
func TimeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	if elapsed > time.Millisecond {
		logThis.Info(fmt.Sprintf("-- %s in %s", name, elapsed), VERBOSESTEST)
	}
}

//-----------------------------------------------------------------------------

type ByteSize float64

const (
	_           = iota // ignore first value by assigning to blank identifier
	KB ByteSize = 1 << (10 * iota)
	MB
	GB
	TB
)

func (b ByteSize) String() string {
	switch {
	case b >= TB:
		return fmt.Sprintf("%.3fTB", b/TB)
	case b >= GB:
		return fmt.Sprintf("%.3fGB", b/GB)
	case b >= MB:
		return fmt.Sprintf("%.3fMB", b/MB)
	case b >= KB:
		return fmt.Sprintf("%.3fKB", b/KB)
	}
	return fmt.Sprintf("%.3fB", b)
}

func readableUInt64(a uint64) string {
	return ByteSize(float64(a)).String()
}
func readableInt64(a int64) string {
	if a >= 0 {
		return "+" + ByteSize(math.Abs(float64(a))).String()
	}
	return "-" + ByteSize(math.Abs(float64(a))).String()
}

func readableInt64Sign(a int64) string {
	if a >= 0 {
		return "+"
	}
	return "-"
}

//-----------------------------------------------------------------------------

// BlueBold outputs a string in blue bold.
func BlueBold(in string) string {
	return chalk.Bold.TextStyle(chalk.Blue.Color(in))
}

// Blue outputs a string in blue.
func Blue(in string) string {
	return chalk.Blue.Color(in)
}

// GreenBold outputs a string in green bold.
func GreenBold(in string) string {
	return chalk.Bold.TextStyle(chalk.Green.Color(in))
}

// Green outputs a string in green.
func Green(in string) string {
	return chalk.Green.Color(in)
}

// RedBold outputs a string in red bold.
func RedBold(in string) string {
	return chalk.Bold.TextStyle(chalk.Red.Color(in))
}

// Red outputs a string in red.
func Red(in string) string {
	return chalk.Red.Color(in)
}

// Yellow outputs a string in yellow.
func Yellow(in string) string {
	return chalk.Yellow.Color(in)
}

// UserChoice message logging
func UserChoice(msg string, args ...interface{}) {
	msg = fmt.Sprintf(msg, args...)
	fmt.Print(BlueBold(msg))
}

// GetInput from user
func GetInput() (string, error) {
	scanner := bufio.NewReader(os.Stdin)
	choice, scanErr := scanner.ReadString('\n')
	return strings.TrimSpace(choice), scanErr
}

// Accept asks a question and returns the answer
func Accept(question string) bool {
	fmt.Printf(BlueBold("%s Y/N : "), question)
	input, err := GetInput()
	if err == nil {
		switch input {
		case "y", "Y", "yes":
			return true
		}
	}
	return false
}

// RemoveDuplicates in []string
func RemoveDuplicates(options *[]string, otherStringsToClean ...string) {
	found := make(map[string]bool)
	// specifically remove other strings from values
	for _, o := range otherStringsToClean {
		found[o] = true
	}
	j := 0
	for i, x := range *options {
		if !found[x] && x != "" {
			found[x] = true
			(*options)[j] = (*options)[i]
			j++
		}
	}
	*options = (*options)[:j]
}

// SelectOption among several, or input a new one, and return user input.
func SelectOption(title, usage string, options []string) (string, error) {
	UserChoice(title)
	if usage != "" {
		fmt.Println(Green(usage))
	}

	// remove duplicates from options and display them
	RemoveDuplicates(&options)
	for i, o := range options {
		fmt.Printf("%d. %s\n", i+1, o)
	}

	errs := 0
	for {
		if len(options) > 1 {
			UserChoice("Choose option [1-%d], or [E]dit: ", len(options))
		} else {
			UserChoice("[E]dit manually, or [A]ccept: ")
		}
		choice, scanErr := GetInput()
		if scanErr != nil {
			return "", scanErr
		}

		if strings.ToUpper(choice) == "E" {
			var edited string
			var scanErr error
			UserChoice("Enter the new value: ")
			edited, scanErr = GetInput()

			if scanErr != nil {
				return "", scanErr
			}
			if edited == "" {
				RedBold("Empty value!")
			} else {
				edited = SanitizeFolder(edited)
				if Accept("Confirm: " + edited) {
					return edited, nil
				}
				RedBold("Not confirmed.")
			}
		} else if strings.ToUpper(choice) == "A" && len(options) == 1 {
			return options[0], nil
		} else if index, err := strconv.Atoi(choice); err == nil && 0 < index && index <= len(options) {
			return options[index-1], nil
		}

		// if we get here, wrong choice
		RedBold("Invalid choice.")
		errs++
		if errs > 10 {
			RedBold("Too many errors")
			return "", errors.New("Invalid choice")
		}

	}
}
