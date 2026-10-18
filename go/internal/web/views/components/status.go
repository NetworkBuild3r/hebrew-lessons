package components

func statusIcon(status string) string {
	switch status {
	case "done":
		return "check"
	case "due":
		return "review"
	case "locked":
		return "lock"
	case "progress":
		return "review"
	default:
		return "today"
	}
}

func statusHe(status string) string {
	switch status {
	case "progress":
		return "בתהליך"
	case "done":
		return "הושלם"
	case "due":
		return "לחזרה"
	case "locked":
		return "נעול"
	default:
		return "חדש"
	}
}

func statusEn(status string) string {
	switch status {
	case "progress":
		return "In progress"
	case "done":
		return "Done"
	case "due":
		return "Due"
	case "locked":
		return "Locked"
	default:
		return "New"
	}
}
