//go:build !unix

package config

import "os"

const secretOpenFlags = os.O_RDONLY
