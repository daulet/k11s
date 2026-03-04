package buildinfo

import "fmt"

var (
	Version = "dev"
	Commit  = "none"
	BuiltAt = "unknown"
)

func Banner(app string, rpcVersion string) string {
	return fmt.Sprintf("%s version=%s commit=%s builtAt=%s rpc=%s", app, Version, Commit, BuiltAt, rpcVersion)
}
