package utils

const (
	WatchByAnnotationKey = "audit.my.domain/watch-by"
	WatchByAnnotationKV  = "watchman"

	WatchActionTypeAnnotationKey = "audit.my.domain/watch-action"
	WatchActionTypeCreate        = "Create"
	WatchActionTypeDelete        = "Delete"
	WatchActionTypeUpdate        = "Update"

	WatchManFieldManager = "watch-man-manager"

	WatchUpdateStateKey = "audit.my.domain/watch-update-state"
	WatchUpdateStateOld = "Old"
	WatchUpdateStateNew = "New"
)

func HasWatchManAnnotation(a map[string]string, key string, val string) bool {
	if kv, ok := a[key]; ok && kv == val {
		return true
	}
	return false
}
