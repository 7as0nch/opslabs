/* *
 * @Author: chengjiang
 * @Date: 2025-10-07 21:35:23
 * @Description:
**/

package agenttools

import (
	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Data struct {
	// TODO wrapped database client
	db *gorm.DB
}
func (d *Data) GetDB() *gorm.DB {
	return d.db
}

// NewData .
func NewData() (*Data, func(), error) {
	cleanup := func() {
	}
	// 初始化mysqldb
	db := NewMysqlDB("read_rentianhua:yNxruv24C26@tcp(rm-bp1hw1ynxruv24c26jo.mysql.rds.aliyuncs.com:3306)/rth?parseTime=True&loc=Local")
	if db == nil {
		log.Error("NewMysqlDB failed")
		return nil, nil, nil
	}
	return &Data{
		db: db,
	}, cleanup, nil
}

func NewMysqlDB(source string) *gorm.DB {
	db, err := gorm.Open(mysql.Open(source), &gorm.Config{
		
	})
	if err != nil {
		log.Error(err)
		return nil
	}
	return db
}