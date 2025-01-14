package utils

const (
	SupportedKindService    string = "Service"
	SupportedKindDeployment string = "Deployment"
)

var supports = map[string]bool{SupportedKindService: true, SupportedKindDeployment: true}

func SupportsAllKinds(kinds ...string) bool {
	for _, kind := range kinds {
		if _, ok := supports[kind]; !ok {
			return false
		}
	}
	return true
}
