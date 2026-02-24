//go:build darwin

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

var origTermios *unix.Termios

func enableRawMode() {
	fd := int(os.Stdin.Fd())
	t, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		panic(err)
	}
	origTermios = t

	raw := *t
	raw.Iflag &^= unix.BRKINT | unix.ICRNL | unix.INPCK | unix.ISTRIP | unix.IXON
	raw.Oflag &^= unix.OPOST
	raw.Cflag |= unix.CS8
	raw.Lflag &^= unix.ECHO | unix.ICANON | unix.IEXTEN | unix.ISIG
	raw.Cc[unix.VMIN] = 0
	raw.Cc[unix.VTIME] = 0

	if err := unix.IoctlSetTermios(fd, unix.TIOCSETA, &raw); err != nil {
		panic(err)
	}
}

func disableRawMode() {
	if origTermios != nil {
		unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TIOCSETA, origTermios)
	}
}
