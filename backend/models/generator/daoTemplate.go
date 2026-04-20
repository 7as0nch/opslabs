package generator

import (
	"os"
	"text/template"
)

type TemplateData struct {
	Package             string
	ModelPath           string
	RepositoryName      string
	ModelVariable       string
	ModelName           string
	ModelVariablePlural string
}

func main() {
	// 从命令行参数中读取模型和数据访问接口的名称
	modelName := os.Args[1]
	repositoryName := os.Args[2]

	// 填充模板变量
	data := TemplateData{
		Package:             "repository",
		ModelPath:           "./model",
		RepositoryName:      repositoryName,
		ModelVariable:       modelName[:1],
		ModelName:           modelName,
		ModelVariablePlural: modelName[:1] + "s",
	}

	// 加载模板文件
	tmpl, err := template.ParseFiles("template.tmpl")
	if err != nil {
		panic(err)
	}

	// 将生成的代码写入文件中
	f, err := os.Create(repositoryName + "_repository.go")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	err = tmpl.Execute(f, data)
	if err != nil {
		panic(err)
	}
}
