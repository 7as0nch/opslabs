package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/example/aichat/backend/models/generator/model"
	myStrings "github.com/example/aichat/backend/tools/strings"
)

type TemplateData struct {
	Package   string
	ModelPath string
	RepoName  string
	ModelName string
}

func main() {
	// 从命令行参数中读取模型和数据访问接口的名称
	// TODO: 这里目前是硬编码的 SysUser，实际使用时应该可以配置或者通过参数传入
	md := model.SysUser{}
	split := strings.Split(md.TableName(), "_")
	sb := myStrings.NewStrBuilder()
	for _, s := range split {
		sb.StrAppend(strings.ToUpper(s[:1]) + s[1:])
	}
	modelName := sb.ToString()
	repoName := strings.ToLower(modelName[:1]) + modelName[1:]

	// 获取当前工作目录
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	// 填充模板变量
	data := TemplateData{
		Package:   "data",
		ModelPath: "github.com/example/aichat/backend/models/generator/model",
		ModelName: modelName,
		RepoName:  repoName,
	}

	var fn = make(chan func(), 2)

	var fn1 = func() {
		// 加载模板文件
		tmplPath := filepath.Join(wd, "cmd", "tmpl", "template.tmpl")
		tmpl, err := template.ParseFiles(tmplPath)
		if err != nil {
			panic(fmt.Errorf("parse template file error: %v, path: %s", err, tmplPath))
		}
		// 将生成的代码写入文件中
		// Use table name for filename to match snake_case convention
		outPath := filepath.Join(wd, "internal", "data", md.TableName()+".go")
		f, err := os.Create(outPath)
		if err != nil {
			panic(err)
		}
		defer f.Close()

		err = tmpl.Execute(f, data)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Generated data file: %s\n", outPath)
	}
	var fn2 = func() {
		// biz层
		tmplPath := filepath.Join(wd, "cmd", "tmpl", "biz.tmpl")
		tmpl, err := template.ParseFiles(tmplPath)
		if err != nil {
			panic(fmt.Errorf("parse template file error: %v, path: %s", err, tmplPath))
		}
		// 将生成的代码写入文件中
		// 注意：biz层现在统一放在 internal/biz/base 下
		outPath := filepath.Join(wd, "internal", "biz", "base", md.TableName()+".go")
		// 确保目录存在
		os.MkdirAll(filepath.Dir(outPath), 0755)

		f, err := os.Create(outPath)
		if err != nil {
			panic(err)
		}
		defer f.Close()

		err = tmpl.Execute(f, data)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Generated biz file: %s\n", outPath)
	}
	fn <- fn1
	fn <- fn2
	//
	var f = <-fn
	var f2 = <-fn
	f()
	f2()
}
