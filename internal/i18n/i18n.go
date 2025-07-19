package i18n

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed lang.json
var langData []byte

var data map[string]map[string]string
var currentLang = "en"

func Load(lang string) error {
	if err := json.Unmarshal(langData, &data); err != nil {
		return err
	}
	if _, ok := data[lang]; ok {
		currentLang = lang
	} else {
		currentLang = "en"
	}
	return nil
}

func T(key string) string {
	if val, ok := data[currentLang][key]; ok {
		return val
	}
	return fmt.Sprintf("??%s??", key)
}
