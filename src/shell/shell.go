package shell

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Opt struct {
	StreamOutputToStdout       bool
	PrintCommand               bool
	StripTrailingNewline       bool
	SuppressStderrStreaming    bool
	CombinedStdoutStderrOutput bool
}

type saveOutput struct {
	savedOutput []byte
}

func (so *saveOutput) Write(p []byte) (n int, err error) {
	so.savedOutput = append(so.savedOutput, p...)
	return os.Stdout.Write(p)
}

func Run(opt Opt, cmdStr string) (string, error) {
	cmd := exec.Command("bash", "-c", cmdStr)

	var (
		out string
		err error
	)
	if opt.PrintCommand {
		fmt.Printf("+ %s\n", cmdStr)
	}
	if opt.SuppressStderrStreaming && opt.CombinedStdoutStderrOutput {
		return "", fmt.Errorf(
			"cannot specify both SuppressStderrStreaming and CombinedStdoutStderrOutput",
		)
	}
	if !opt.SuppressStderrStreaming && !opt.CombinedStdoutStderrOutput {
		cmd.Stderr = os.Stderr
	}
	if opt.StreamOutputToStdout {
		var so saveOutput
		cmd.Stdout = &so
		err = cmd.Run()
		out = string(so.savedOutput)
	} else {
		var outBytes []byte
		var cmdErr error
		if opt.CombinedStdoutStderrOutput {
			outBytes, cmdErr = cmd.CombinedOutput()
		} else {
			outBytes, cmdErr = cmd.Output()
		}
		out = string(outBytes)
		err = cmdErr
	}
	if err != nil {
		return out, fmt.Errorf("running command: %q: %w", cmdStr, err)
	}
	if opt.StripTrailingNewline {
		out = strings.TrimRight(out, "\n")
	}
	return out, nil
}

func RunAndCollectLines(opt Opt, cmdStr string) ([]string, error) {
	opt.StripTrailingNewline = true
	out, err := Run(opt, cmdStr)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	lines := strings.Split(out, "\n")
	return lines, nil
}

func OpenEditor(
	template string,
	filePrefix string,
	commentChar string,
) (string, error) {
	tmpDir := os.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, filePrefix)
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	_, err = tmpFile.WriteString(template)
	if err != nil {
		return "", fmt.Errorf("writing template to temp file %q: %s", tmpFile.Name(), err)
	}

	editor := os.Getenv("GIT_EDITOR")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}

	cmdStr := fmt.Sprintf("%s %s", editor, tmpFile.Name())
	cmd := exec.Command("bash", "-c", cmdStr)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Print("hint: Waiting for your editor to close the file...")
	defer fmt.Println()

	err = cmd.Start()
	if err != nil {
		return "", fmt.Errorf("starting command `%s`: %w", cmdStr, err)
	}

	err = cmd.Wait()
	if err != nil {
		return "", fmt.Errorf("running command `%s`: %w", cmdStr, err)
	}

	newContent, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("reading temp file %q after user edit: %w", tmpFile.Name(), err)
	}

	var result []string
	for _, line := range strings.Split(string(newContent), "\n") {
		if !strings.HasPrefix(line, commentChar) {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n"), nil
}
