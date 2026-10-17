package job

import "golang.org/x/sys/windows/registry"

// steamPathFromRegistry returns where Steam is installed, or "" if unknown.
func steamPathFromRegistry() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	p, _, err := k.GetStringValue("SteamPath")
	if err != nil {
		return ""
	}
	return p
}
