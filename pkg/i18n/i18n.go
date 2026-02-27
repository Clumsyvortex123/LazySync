package i18n

// TranslationSet holds all UI strings
type TranslationSet struct {
	AppName            string
	Version            string
	ErrorTitle         string
	CannotKillChildError string
	Yes                string
	No                 string
	Confirm            string
	Cancel             string
	Close              string
	Filter             string
	Navigate           string
	Scroll             string
	Execute            string
	Open               string
	Return             string
}

// NewEnglishTranslations returns English translations
func NewEnglishTranslations() *TranslationSet {
	return newEnglish()
}
