package main

import (
	"flag"
	"strings"
	
	"gorm.io/gen"
	"gorm.io/gorm"
	
	"github.com/Wenrh2004/sandbox/internal/task/infrastructure/repository"
	"github.com/Wenrh2004/sandbox/pkg/application/config"
	"github.com/Wenrh2004/sandbox/pkg/bootstrap"
	"github.com/Wenrh2004/sandbox/pkg/log"
)

const (
	queryPath = "./internal/task/infrastructure/repository/query"
	modelPath = "./internal/task/infrastructure/model"
)

func main() {
	var envConf = flag.String("conf", "config/bootstrap.yml", "config path, eg: -conf ./config/bootstrap.yml")
	flag.Parse()
	boot := bootstrap.NewBootstrap(*envConf)
	conf := config.NewConfig(boot).GetConfig()
	
	logger := log.NewLog(conf)
	
	g := gen.NewGenerator(gen.Config{
		OutPath:          queryPath,
		ModelPkgPath:     modelPath,
		Mode:             gen.WithoutContext | gen.WithDefaultQuery | gen.WithQueryInterface, // generate mode
		FieldSignable:    true,
		FieldWithTypeTag: true,
		FieldNullable:    true,
	})
	
	gormDB := repository.NewDB(conf, logger)
	g.UseDB(gormDB) // reuse your gorm db
	
	var dataMap = map[string]func(gorm.ColumnType) (dataType string){
		"bigint": func(columnType gorm.ColumnType) (dataType string) {
			if n, ok := columnType.Nullable(); ok && n {
				return "*int64"
			}
			if n, ok := columnType.AutoIncrement(); ok && n {
				return "uint"
			}
			if n, ok := columnType.PrimaryKey(); ok && n {
				return "uint64"
			}
			return "uint64"
		},
		
		// bool mapping
		"tinyint": func(columnType gorm.ColumnType) (dataType string) {
			ct, _ := columnType.ColumnType()
			if strings.HasPrefix(ct, "tinyint(1)") {
				return "bool"
			}
			return "byte"
		},
		
		"date": func(columnType gorm.ColumnType) (dataType string) {
			return "*time.Time"
		},
		
		"json": func(columnType gorm.ColumnType) (dataType string) {
			return "[]byte"
		},
	}
	
	g.WithDataTypeMap(dataMap)
	
	// isDeleteField := gen.FieldGORMTag("is_deleted", func(tag field.GormTag) field.GormTag {
	// 	return field.GormTag{}.Set("column", "is_deleted").Set("type", "tinyint(1)").Set("", "not null").Set("softDelete", "flag").Set("DeletedAtField", "DeletedAt").Set("comment", "是否删除")
	// })
	// softDeleteField := gen.FieldType("is_deleted", "soft_delete.DeletedAt")
	// idField := gen.FieldGORMTag("id", func(tag field.GormTag) field.GormTag {
	// 	return field.GormTag{}.Set("column", "id").Set("type", "bigint(20)").Set("", "not null").Set("primaryKey", "flag").Set("autoIncrement", "flag").Set("comment", "主键")
	// })
	// sidField := gen.FieldType("id", "uint64")
	// fieldOpts := []gen.ModelOpt{isDeleteField, softDeleteField, idField, sidField}
	
	g.ApplyBasic(
		// g.GenerateAllTable(fieldOpts...),
		g.GenerateModel("submit_infos"),
		g.GenerateModel("task_infos"),
	)
	
	// Generate the code
	g.Execute()
}
