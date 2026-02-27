package i18n

// newEnglish returns the English translation set
func newEnglish() *TranslationSet {
	return &TranslationSet{
		AppName:              "lazyscpsync",
		Version:              "0.1.0",
		ErrorTitle:           "Error",
		CannotKillChildError: "Cannot kill child process",
		Yes:                  "Yes",
		No:                   "No",
		Confirm:              "Confirm",
		Cancel:               "Cancel",
		Close:                "Close",
		Filter:               "Filter",
		Navigate:             "Navigate",
		Scroll:               "Scroll",
		Execute:              "Execute",
		Open:                 "Open",
		Return:               "Return",
	}
}
