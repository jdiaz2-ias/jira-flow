package cli

import (
	"context"
	"errors"
	"io"
	"os"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
	"jira-flow.local/jflow/internal/domain"
)

// Poll terminal input so setup prompts respect cancellation without orphaning a
// goroutine that might later consume input intended for the TUI.
func terminalByte(ctx context.Context, f *os.File, b []byte) (int, error) {
	for {
		if ctx.Err() != nil {
			return 0, &domain.Error{Kind: domain.Canceled, Message: "Input canceled.", Cause: ctx.Err()}
		}
		ready := []unix.PollFd{{Fd: int32(f.Fd()), Events: unix.POLLIN}}
		n, err := unix.Poll(ready, 100)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if n == 0 {
			continue
		}
		n, err = unix.Read(int(f.Fd()), b)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if n == 0 && err == nil {
			err = io.EOF
		}
		return n, err
	}
}
func readHiddenToken(ctx context.Context, f *os.File) (value []byte, err error) {
	state, err := term.MakeRaw(int(f.Fd()))
	if err != nil {
		return nil, err
	}
	defer func() {
		if restoreErr := term.Restore(int(f.Fd()), state); err == nil {
			err = restoreErr
		}
	}()
	one := make([]byte, 1)
	for len(value) <= 65536 {
		_, err = terminalByte(ctx, f, one)
		if err != nil {
			return nil, err
		}
		switch one[0] {
		case '\r', '\n':
			return value, nil
		case 3:
			return nil, &domain.Error{Kind: domain.Canceled, Message: "Input canceled."}
		case 4:
			return nil, &domain.Error{Kind: domain.Canceled, Message: "Input ended."}
		case 8, 127:
			if len(value) > 0 {
				value = value[:len(value)-1]
			}
		default:
			if one[0] >= 32 {
				value = append(value, one[0])
			}
		}
	}
	return nil, &domain.Error{Kind: domain.InvalidInput, Message: "Token exceeds 64 KiB."}
}
