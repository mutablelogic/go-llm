package schema

import (
	"strings"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// whisperLanguages are the ISO 639-1 codes of the languages whisper
// detects, by the names it returns them as
var whisperLanguages = map[string]string{
	"afrikaans": "af", "albanian": "sq", "amharic": "am", "arabic": "ar", "armenian": "hy", "assamese": "as",
	"azerbaijani": "az", "bashkir": "ba", "basque": "eu", "belarusian": "be", "bengali": "bn", "bosnian": "bs",
	"breton": "br", "bulgarian": "bg", "catalan": "ca", "chinese": "zh", "croatian": "hr", "czech": "cs",
	"danish": "da", "dutch": "nl", "english": "en", "estonian": "et", "faroese": "fo", "finnish": "fi",
	"french": "fr", "galician": "gl", "georgian": "ka", "german": "de", "greek": "el", "gujarati": "gu",
	"haitian creole": "ht", "hausa": "ha", "hebrew": "he", "hindi": "hi", "hungarian": "hu", "icelandic": "is",
	"indonesian": "id", "italian": "it", "japanese": "ja", "javanese": "jw", "kannada": "kn", "kazakh": "kk",
	"khmer": "km", "korean": "ko", "lao": "lo", "latin": "la", "latvian": "lv", "lingala": "ln",
	"lithuanian": "lt", "luxembourgish": "lb", "macedonian": "mk", "malagasy": "mg", "malay": "ms", "malayalam": "ml",
	"maltese": "mt", "maori": "mi", "marathi": "mr", "mongolian": "mn", "myanmar": "my", "nepali": "ne",
	"norwegian": "no", "nynorsk": "nn", "occitan": "oc", "pashto": "ps", "persian": "fa", "polish": "pl",
	"portuguese": "pt", "punjabi": "pa", "romanian": "ro", "russian": "ru", "sanskrit": "sa", "serbian": "sr",
	"shona": "sn", "sindhi": "sd", "sinhala": "si", "slovak": "sk", "slovenian": "sl", "somali": "so",
	"spanish": "es", "sundanese": "su", "swahili": "sw", "swedish": "sv", "tagalog": "tl", "tajik": "tg",
	"tamil": "ta", "tatar": "tt", "telugu": "te", "thai": "th", "tibetan": "bo", "turkish": "tr",
	"turkmen": "tk", "ukrainian": "uk", "urdu": "ur", "uzbek": "uz", "vietnamese": "vi", "welsh": "cy",
	"yiddish": "yi", "yoruba": "yo",
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// LanguageCode returns the ISO 639-1 code of a language, by the name whisper
// returns it as, such as "fr" for "french", in any case, or the language, in
// lower case, when it isn't one of whisper's, such as a code, or an empty
// string when there's none
func LanguageCode(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	if code, exists := whisperLanguages[language]; exists {
		return code
	}
	return language
}
