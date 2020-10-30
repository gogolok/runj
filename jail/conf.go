package jail

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"
)

const (
	defaultStateDir = "/var/lib/runj/jails"
	stateDir        = defaultStateDir
	confName        = "jail.conf"
	configTemplate  = `{{ .Name }} {
  path = "{{ .Root }}";
  persist;
}
`
)

func CreateConfig(id, root string) (string, error) {
	config, err := renderConfig(id, root)
	if err != nil {
		return "", err
	}
	fmt.Println(config)
	jailPath := filepath.Join(stateDir, id)
	err = os.MkdirAll(jailPath, 0755)
	if err != nil {
		return "", err
	}
	confPath := ConfPath(id)
	if _, err := os.Stat(confPath); err == nil {
		return "", errors.New("config should not already exist")
	}
	return confPath, ioutil.WriteFile(confPath, []byte(config), 0644)
}

func ConfPath(id string) string {
	return filepath.Join(stateDir, id, confName)
}

func renderConfig(id, root string) (string, error) {
	config, err := template.New("config").Parse(configTemplate)
	if err != nil {
		return "", err
	}
	buf := bytes.Buffer{}
	config.Execute(&buf, struct {
		Name string
		Root string
	}{
		Name: id,
		Root: root,
	})
	return buf.String(), nil
}

func RemoveConfig(id string) error {
	return os.RemoveAll(filepath.Join(stateDir, id))
}