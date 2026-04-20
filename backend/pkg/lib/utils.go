package lib

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"go.mongodb.org/mongo-driver/mongo"
)

const IPPrefix = "192.168.194"

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

//Getenv 获取环境变量的值 如果为空则将第二个参数作为默认值
func Getenv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

func substr(s string, pos, length int) string {
	runes := []rune(s)
	l := pos + length
	if l > len(runes) {
		l = len(runes)
	}
	return string(runes[pos:l])
}

//GetParentDirectory 获得父目录
func GetParentDirectory(dirctory string) string {
	return substr(dirctory, 0, strings.LastIndex(dirctory, "/"))
}

//GetCurrentDirectory 获得当前程序目录
func GetCurrentDirectory() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		// logging.GetLogger().Error(err)
	}
	return strings.Replace(dir, "\\", "/", -1)
}

//GetLocalIP 获取本机IP地址
func GetLocalIP() {
	addrs, err := net.InterfaceAddrs()

	if err != nil {
		os.Stderr.WriteString("Oops:" + err.Error())
		os.Exit(1)
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {

				// result.append(ipnet.IP.String())
			}
		}
	}
	os.Exit(0)
}

//Base64FromFile 读入文件并使用Base64编码 并且计算MD5
func Base64FromFile(path string) (string, string, error) {

	content, err := ioutil.ReadFile(path)

	if err != nil {
		return "", "", err
	}

	encoded := base64.StdEncoding.EncodeToString(content)
	hasher := md5.New()
	hasher.Write(content)
	md5String := hex.EncodeToString(hasher.Sum(nil))

	return encoded, md5String, nil
}

//CreateCaptcha 产生跟时间戳相关的随机数字符串 bit代表位数
func CreateCaptcha(bit int) string {
	return fmt.Sprintf("%"+strconv.Itoa(bit)+"v", rand.New(rand.NewSource(time.Now().UnixNano())).Int31n(1000000))
}

//RandStringRunes 随机
func RandStringRunes(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

//DecimalOneDigit 生成报告中格式化一位小数的字符串带百分号
func DecimalOneDigit(value float64) string {
	return fmt.Sprintf("%.1f", value*100) + "%"
}

//CopyStruct 结构体深度复制
func CopyStruct(src, dst interface{}) {
	sval := reflect.ValueOf(src).Elem()
	dval := reflect.ValueOf(dst).Elem()

	for i := 0; i < sval.NumField(); i++ {
		value := sval.Field(i)
		name := sval.Type().Field(i).Name

		dvalue := dval.FieldByName(name)
		if dvalue.IsValid() == false {
			continue
		}
		dvalue.Set(value) //这里默认共同成员的类型一样，否则这个地方可能导致 panic，需要简单修改一下。
	}
}

//ConvertFtoS 将浮点数转化为字符串 原封不动的转
func ConvertFtoS(src float64) string {
	return fmt.Sprintf("%f\n", src)
}

//UploadMultipartFile 上传文件
func UploadMultipartFile(client *http.Client, uri, key, path string, headers map[string]string) (*http.Response, error) {
	body, writer := io.Pipe()

	req, err := http.NewRequest(http.MethodPost, uri, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("haha", "hah")
	mwriter := multipart.NewWriter(writer)
	req.Header.Add("Content-Type", mwriter.FormDataContentType())
	for key, value := range headers {
		fmt.Println(key)
		fmt.Println(value)

		req.Header.Add(key, value)
	}

	errchan := make(chan error)

	go func() {
		defer close(errchan)
		defer writer.Close()
		defer mwriter.Close()

		w, err := mwriter.CreateFormFile(key, path)
		if err != nil {
			errchan <- err
			return
		}

		in, err := os.Open(path)
		if err != nil {
			errchan <- err
			return
		}
		defer in.Close()

		if written, err := io.Copy(w, in); err != nil {
			errchan <- fmt.Errorf("error copying %s (%d bytes written): %v", path, written, err)
			return
		}

		if err := mwriter.Close(); err != nil {
			errchan <- err
			return
		}
	}()

	resp, err := client.Do(req)
	merr := <-errchan

	if err != nil || merr != nil {
		return resp, fmt.Errorf("http error: %v, multipart error: %v", err, merr)
	}

	return resp, nil
}

//HealthCheckHandler 构造httptest http头
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	// A very simple health check.
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")

	// In the future we could report back on the status of our DB, or our cache
	// (e.g. Redis) by performing a simple PING, and include them in the response.
	io.WriteString(w, `{"alive": true}`)
}

//EnsureDir 确保路径存在
func EnsureDir(dirName string) error {

	err := os.Mkdir(dirName, os.ModeDir)

	if err == nil || os.IsExist(err) {
		return nil
	}
	return err

}

//VerifyEmailFormat 验证邮箱格式是否正确
func VerifyEmailFormat(email string) bool {
	pattern := `\w+([-+.]\w+)*@\w+([-.]\w+)*\.\w+([-.]\w+)*` //匹配电子邮箱
	reg := regexp.MustCompile(pattern)
	return reg.MatchString(email)
}

//VerifyMobileFormat 验证手机号格式是否正确
func VerifyMobileFormat(mobileNum string) bool {
	regular := "^((13[0-9])|(14[5,7])|(15[0-3,5-9])|(17[0,3,5-8])|(18[0-9])|166|198|199|(147))\\d{8}$"

	reg := regexp.MustCompile(regular)
	return reg.MatchString(mobileNum)
}

//VeriyfiMongoErr 验证是否为空
//为空返回true
//不是空错误返回false
func VerifyMongoErr(err error) bool {
	if err == nil {
		return true
	}
	if err == mongo.ErrNilDocument || err == mongo.ErrNilCursor || err == mongo.ErrNoDocuments {
		return true
	}
	return false
}

// 判断是不是真实手机号码
func IsMobile(mobile string) bool {
	result, _ := regexp.MatchString(`^(1\d{10})$`, mobile)
	if result {
		return true
	} else {
		return false
	}
}

func RandStringBytesMaskImprSrcUnsafe(n int) string {

	var src = rand.NewSource(time.Now().UnixNano())
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return *(*string)(unsafe.Pointer(&b))
}

func GetIP() string {
	ips := IPS()

	for _, ip := range ips {
		if strings.Contains(ip, IPPrefix) {
			return ip
		}
	}
	return ""
}

func ExternalIP() (res []string) {
	inters, err := net.Interfaces()
	if err != nil {
		return
	}
	for _, inter := range inters {
		if !strings.HasPrefix(inter.Name, "lo") {
			addrs, err := inter.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok {
					if ipnet.IP.IsLoopback() || ipnet.IP.IsLinkLocalMulticast() || ipnet.IP.IsLinkLocalUnicast() {
						continue
					}
					if ip4 := ipnet.IP.To4(); ip4 != nil {
						switch true {
						case ip4[0] == 10:
							continue
						case ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31:
							continue
						case ip4[0] == 192 && ip4[1] == 168:
							continue
						default:
							res = append(res, ipnet.IP.String())
						}
					}
				}
			}
		}
	}
	return
}

// isUp Interface is up
func isUp(v net.Flags) bool {
	return v&net.FlagUp == net.FlagUp
}

func IPS() (ips []string) {
	inters, err := net.Interfaces()
	if err != nil {
		return
	}
	for _, inter := range inters {
		if !isUp(inter.Flags) {
			continue
		}
		if !strings.HasPrefix(inter.Name, "lo") {
			addrs, err := inter.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						ips = append(ips, ipnet.IP.To4().String())
					}
				}
			}
		}
	}
	return
}

//IsDev 根据环境变量cloudbrainMode判断当前是否为生产环境或者开发环境
func IsDev() bool {
	mode := os.Getenv("cloudbrainMode")
	if mode == "" {
		log.Fatal("需要配置cloudbrainMode环境变量")
		return true
	}
	if mode == dev {
		return true
	}

	return false
}

//GetType 获得类型的名称
func GetTypeFullName(myvar interface{}) string {
	if t := reflect.TypeOf(myvar); t.Kind() == reflect.Ptr {
		return t.Elem().PkgPath() + "/" + t.Elem().Name()
	} else {
		return t.PkgPath() + "/" + t.Name()
	}
}

//Exists 判断给定路径的文件是否存在
func Exists(path string) bool {
	_, err := os.Stat(path) //os.Stat获取文件信息
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}

//GetCurrentAbPathByCaller 获取当前项目绝对路径
func GetCurrentAbPathByCaller() string {
	var abPath string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		abPath = path.Dir(filename)
	}
	return abPath
}

// func GetModuleName(basePath string ) string {
// 	basePath := GetCurrentAbPathByCaller()
// 	goModFile := filepath.Join(basePath, "go.mod")
// 	goModBytes, err := ioutil.ReadFile(goModFile)
// 	if err != nil {
// 		panic(err)
// 	}
// 	modName := modfile.ModulePath(goModBytes)
// 	fmt.Fprintf(os.Stdout, "modName=%+v\n", modName)

// 	return modName
// }
