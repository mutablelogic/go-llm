package elevenlabs

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// languages are the ISO 639-1 codes of the languages ElevenLabs transcribes,
// by the ISO 639-3 codes it returns them as, for those which have one
var languages = map[string]string{
	"afr": "af", "amh": "am", "ara": "ar", "hye": "hy", "asm": "as", "aze": "az", "bel": "be", "ben": "bn",
	"bos": "bs", "bul": "bg", "mya": "my", "cat": "ca", "nya": "ny", "hrv": "hr", "ces": "cs", "dan": "da",
	"nld": "nl", "eng": "en", "est": "et", "fin": "fi", "fra": "fr", "ful": "ff", "glg": "gl", "lug": "lg",
	"kat": "ka", "deu": "de", "ell": "el", "guj": "gu", "hau": "ha", "heb": "he", "hin": "hi", "hun": "hu",
	"isl": "is", "ibo": "ig", "ind": "id", "gle": "ga", "ita": "it", "jpn": "ja", "jav": "jv", "kan": "kn",
	"kaz": "kk", "khm": "km", "kor": "ko", "kur": "ku", "kir": "ky", "lao": "lo", "lav": "lv", "lin": "ln",
	"lit": "lt", "ltz": "lb", "mkd": "mk", "msa": "ms", "mal": "ml", "mlt": "mt", "cmn": "zh", "zho": "zh",
	"mri": "mi", "mar": "mr", "mon": "mn", "nep": "ne", "nor": "no", "oci": "oc", "ori": "or", "pus": "ps",
	"fas": "fa", "pol": "pl", "por": "pt", "pan": "pa", "ron": "ro", "rus": "ru", "srp": "sr", "sna": "sn",
	"snd": "sd", "slk": "sk", "slv": "sl", "som": "so", "spa": "es", "swa": "sw", "swe": "sv", "tam": "ta",
	"tgk": "tg", "tel": "te", "tha": "th", "tur": "tr", "ukr": "uk", "urd": "ur", "uzb": "uz", "vie": "vi",
	"cym": "cy", "wol": "wo", "xho": "xh", "zul": "zu",
}
