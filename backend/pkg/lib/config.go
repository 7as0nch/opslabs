//我们的配置的最终存储方式为 Key-> key->value的形式  以key _prod _dev为后缀来区分生产环境与开发环境
//在生产环境中与开发环境中配置文件的写法完全不同
//在开发环境中为一个一个的yaml结构的configmap文件，在内存中的存储结构为map[string]ConfigMap，其中Key为configMap的路径文件名
//在生产环境中，配置文件为简单map[string]map[string]string 其中第一个Key为文件名，第二个Key为配置名 第三个就为配置值

package lib

import (
	"bufio"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
)

const (
	dev  = "dev"
	prod = "prod"
)

type Config interface {
	Get(string) string
	GetInt(string) int
	GetBool(string) bool
}

var configDatas ConfigMap
var configFileRead map[string]bool

type ConfigMap map[string]map[string]string

func init() {
	configDatas = make(ConfigMap)
	configFileRead = map[string]bool{}
}

type metaData struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type configMapDesc struct {
	Kind       string                 `yaml:"kind"`
	ApiVersion string                 `yaml:"apiVersion"`
	MetaData   metaData               `yaml:"metadata"`
	Data       map[string]interface{} `yaml:"data"`
}

//ReadConfigMap 将configmap 读取到 内存
func ReadConfigMap(filePath string) Config {
	if IsDev() == true {
		return ReadConfigMapDev(filePath)
	} else {
		return ReadConfigMapProd(filePath)
	}
}

func ReadConfigMapDev(filePath string) Config {
	if _, ok := configFileRead[filePath]; ok {
		return &configDatas
	}

	config := configMapDesc{}
	file, err := os.Open(filePath)

	if err != nil {
		panic(err)
	}
	defer file.Close()
	data, _ := ioutil.ReadAll(file)

	err = yaml.Unmarshal([]byte(data), &config)

	if err != nil {
		panic(err)
	}

	for key, values := range config.Data {
		settingsArray := strings.Split(values.(string), "\n")
		settingsMap := map[string]string{}

		for _, settings := range settingsArray {
			if settings == "" {
				continue
			}
			items := strings.Split(settings, "=")
			if len(items) > 1 {
				settingsMap[items[0]] = items[1]
			}
		}
		configDatas[key] = settingsMap
	}

	configFileRead[filePath] = true

	return &configDatas
}

func ReadConfigMapProd(filePath string) Config {

	file, err := os.Open(filePath)

	if err != nil {
		panic(err)
	}
	defer file.Close()

	fileName := filepath.Base(filePath)

	settings := map[string]string{}

	br := bufio.NewReader(file)
	for {
		a, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		settingsLine := string(a)

		items := strings.Split(settingsLine, "=")
		if len(items) > 1 {
			settings[items[0]] = items[1]
		}
	}
	configDatas[fileName] = settings
	return &configDatas
}

//Get 获取configMap中的配置 key格式为 module.key比如DB中的mysql
//Key为 DB.mysql-URL
func (c *ConfigMap) Get(key string) string {

	keyArray := strings.Split(key, ".")

	if len(keyArray) < 2 {
		panic("key format must be module.key")
	}

	module := keyArray[0]

	var realKey string
	if IsDev() == true {
		realKey = keyArray[1] + "-dev"
	} else {
		realKey = keyArray[1] + "-prod"
	}

	settingsMap := (map[string]map[string]string)(*c)

	if settings, ok := settingsMap[module]; !ok {
		panic("module not found.Check your module input")
	} else {
		if value, ok := settings[realKey]; !ok {
			if value, ok = settings[keyArray[1]]; !ok {
				panic("key not found.Check your settings")
			} else {
				return value
			}
		} else {
			return value
		}
	}
}

func (c *ConfigMap) GetInt(key string) int {
	value := c.Get(key)
	if data, err := strconv.Atoi(value); err != nil {
		panic(err)
	} else {
		return data
	}

}

func (c *ConfigMap) GetBool(key string) bool {
	value := c.Get(key)
	if data, err := strconv.ParseBool(value); err != nil {
		panic(err)
	} else {
		return data
	}
}
