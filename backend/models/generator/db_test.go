package generator

import (
	"testing"

	"github.com/7as0nch/backend/models/generator/model"
	"gorm.io/driver/postgres"
	"gorm.io/gen"
	"gorm.io/gorm"
)

// PostgreSQL连接配置 - 使用项目中的配置
const URL = "host=sshjd.aihelper.chat port=30532 user=pgadmin password=pgcj123456 dbname=pgdb sslmode=disable search_path=opslabs"

// Dynamic SQL
type Querier interface {
	// SELECT * FROM @@table WHERE name = @name{{if role !=""}} AND role = @role{{end}}
	FilterWithNameAndRole(name, role string) ([]gen.T, error)
}

func TestDb(t *testing.T) {
	g := gen.NewGenerator(gen.Config{
		OutPath: "./query",
		Mode:    gen.WithoutContext | gen.WithDefaultQuery | gen.WithQueryInterface, // generate mode gen.WithoutContext |
		// FieldNullable:  true,
		// FieldCoverable: false,
		// WithUnitTest:   false, // 不生成单元测试
	})

	// 连接PostgreSQL数据库
	gormdb, _ := gorm.Open(postgres.Open(URL))
	g.UseDB(gormdb) // reuse your gorm db

	// PostgreSQL数据类型映射
	// dataMap := map[string]func(columnType gorm.ColumnType) (dataType string){
	// 	"smallint":         func(columnType gorm.ColumnType) (dataType string) { return "int64" },
	// 	"integer":          func(columnType gorm.ColumnType) (dataType string) { return "int64" },
	// 	"bigint":           func(columnType gorm.ColumnType) (dataType string) { return "int64" },
	// 	"numeric":          func(columnType gorm.ColumnType) (dataType string) { return "float64" },
	// 	"double precision": func(columnType gorm.ColumnType) (dataType string) { return "float64" },
	// 	"float4":           func(columnType gorm.ColumnType) (dataType string) { return "float32" },
	// }
	// g.WithDataTypeMap(dataMap)

	// PostgreSQL时间字段配置和其他生成选项将在GenerateAllTable中自动处理

	// 生成特定表和所有表的模型 (PostgreSQL schema: aichat)
	// 注意：GORM gen在当前版本中可能不支持WithSchema方法

	//Generate basic type-safe DAO API for struct `model.User` following conventions
	var models = []interface{}{
		model.SysUser{},
		model.SysUserAuth{},
		model.SysMenu{},
		model.SysDict{},
		model.SysDictType{},
		model.SysTracker{},
		model.OpslabsAttempt{},
	}
	g.ApplyBasic(models...)
	g.ApplyInterface(func(Querier) {}, models...)
	// Generate the code
	g.Execute()
	t.Log("PostgreSQL数据库表结构生成成功，schema: aichat")
}

// CodeFirst: 将model下面的User生成到数据库表里去。
// PostgreSQL数据库生成脚本 - 使用schema: aichat
func TestMigrate(t *testing.T) {
	// struct to db
	// 连接PostgreSQL数据库
	gormdb, _ := gorm.Open(postgres.Open(URL))
	// 使用schema: aichat
	// gormdb.Migrator().AutoMigrate(&model.SysMenu{})
	// var modelInterface ModelInterface = &model.SysMenu{}

	err := gormdb.Migrator().AutoMigrate(
	// model.AIAgent{},
	// model.AITool{},
	// model.AIToolAgentBind{},
	// model.AIModel{},
	// model.AIPromptTemplate{},
	// model.AIWorkflow{},
	// model.AIApplication{},
	model.OpslabsAttempt{},
	)
	if err != nil {
		t.Logf("迁移失败: %v", err)
		return
	}
	// // 1. 创建备份表并复制数据
	// err := gormdb.Exec(fmt.Sprintf("CREATE TABLE %s_backup AS SELECT * FROM %s",
	//     modelInterface.TableName(), modelInterface.TableName())).Error
	// if err != nil {
	//     t.Logf("备份失败: %v", err)
	//     return
	// }
	// gormdb.Migrator().DropTable(modelInterface)
	// gormdb.Migrator().CreateTable(modelInterface)
	// // 4. 恢复数据
	// gormdb.Exec(fmt.Sprintf("INSERT INTO %s SELECT * FROM %s_backup",
	//     modelInterface.TableName(), modelInterface.TableName()))

	// // 5. 清理备份表
	// gormdb.Exec(fmt.Sprintf("DROP TABLE %s_backup", modelInterface.TableName()))

	t.Log("安全迁移完成，数据已保留")
}

type ModelInterface interface {
	TableName() string
}
