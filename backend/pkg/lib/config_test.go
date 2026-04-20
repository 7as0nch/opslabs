//我们的配置的最终存储方式为 Key-> key->value的形式  以key _prod _dev为后缀来区分

//生产环境与开发环境

package lib

import (
	"fmt"
	"testing"
)

func TestReadConfigMap(t *testing.T) {
	type args struct {
		filePath string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
		{
			name: "test",
			args: args{
				filePath: "../test/config.yaml",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configs := ReadConfigMap(tt.args.filePath)
			fmt.Println(configs.Get("db.mongo-URL"))
		})
	}
}
