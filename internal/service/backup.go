package service

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"myblog/internal/model"
	"myblog/internal/util"
)

// Backup preserves the Java backup API: "attach" writes attachment/theme zip
// files to an existing server directory; "db" writes a short-lived SQL zip
// under the public upload directory.
func (s *Service) Backup(backupType, targetDirectory, themeDirectory string) (*model.BackResponseBo, error) {
	switch backupType {
	case "attach":
		if targetDirectory == "" {
			return nil, Tip("请输入备份文件存储路径")
		}
		info, err := os.Stat(targetDirectory)
		if err != nil || !info.IsDir() {
			return nil, Tip("请输入一个存在的目录")
		}
		name := time.Now().Format("200601021504") + "_" + util.RandomNumber(5) + ".zip"
		attachPath := filepath.Join(targetDirectory, "attachs_"+name)
		themePath := filepath.Join(targetDirectory, "themes_"+name)
		if err := zipDirectory(s.cfg.UploadDir, attachPath); err != nil {
			return nil, err
		}
		if err := zipDirectory(themeDirectory, themePath); err != nil {
			return nil, err
		}
		return &model.BackResponseBo{AttachPath: attachPath, ThemePath: themePath}, nil
	case "db":
		if err := os.MkdirAll(s.cfg.UploadDir, 0o755); err != nil {
			return nil, err
		}
		name := "tale_" + time.Now().Format("200601021504") + "_" + util.RandomNumber(5) + ".sql.zip"
		path := filepath.Join(s.cfg.UploadDir, name)
		if err := s.writeSQLBackup(path); err != nil {
			return nil, err
		}
		time.AfterFunc(10*time.Second, func() { _ = os.Remove(path) })
		return &model.BackResponseBo{SqlPath: "/upload/" + name}, nil
	default:
		return nil, Tip("不支持的备份类型")
	}
}

func (s *Service) writeSQLBackup(target string) error {
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	archive := zip.NewWriter(file)
	entry, err := archive.Create(strings.TrimSuffix(filepath.Base(target), ".zip"))
	if err != nil {
		return err
	}
	for _, table := range []string{
		"t_users", "t_options", "t_contents", "t_metas",
		"t_relationships", "t_comments", "t_attach", "t_logs",
	} {
		if err := s.dumpTable(entry, table); err != nil {
			return err
		}
	}
	return archive.Close()
}

func (s *Service) dumpTable(writer io.Writer, table string) error {
	ddl, err := s.tableDDL(table)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "\n-- %s\nDROP TABLE IF EXISTS `%s`;\n%s;\n", table, table, ddl); err != nil {
		return err
	}
	rows, err := s.db.Table(table).Rows()
	if err != nil {
		return err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return err
	}
	for rows.Next() {
		values, pointers := scanValues(len(columns))
		if err := rows.Scan(pointers...); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(
			writer,
			"INSERT INTO `%s` (`%s`) VALUES (%s);\n",
			table,
			strings.Join(columns, "`,`"),
			sqlValues(values),
		); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *Service) tableDDL(table string) (string, error) {
	if s.db.Dialector.Name() == "mysql" {
		var tableName, ddl string
		row := s.db.Raw("SHOW CREATE TABLE `" + table + "`").Row()
		if err := row.Scan(&tableName, &ddl); err != nil {
			return "", err
		}
		return ddl, nil
	}
	var ddl string
	err := s.db.Raw(
		"SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?",
		table,
	).Scan(&ddl).Error
	if err != nil || ddl == "" {
		return "", fmt.Errorf("read schema for %s: %w", table, err)
	}
	return ddl, nil
}

func scanValues(size int) ([]any, []any) {
	values := make([]any, size)
	pointers := make([]any, size)
	for index := range values {
		pointers[index] = &values[index]
	}
	return values, pointers
}

func sqlValues(values []any) string {
	encoded := make([]string, len(values))
	for index, value := range values {
		encoded[index] = sqlValue(value)
	}
	return strings.Join(encoded, ",")
}

func sqlValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "NULL"
	case []byte:
		return quoteSQL(string(typed))
	case string:
		return quoteSQL(typed)
	case bool:
		if typed {
			return "1"
		}
		return "0"
	case int64:
		return strconv.FormatInt(typed, 10)
	case float64:
		return strconv.FormatFloat(typed, 'g', -1, 64)
	case time.Time:
		return quoteSQL(typed.Format("2006-01-02 15:04:05"))
	default:
		return quoteSQL(fmt.Sprint(value))
	}
}

func quoteSQL(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func zipDirectory(source, target string) error {
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	archive := zip.NewWriter(file)
	err = filepath.Walk(source, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		entry, err := archive.Create(filepath.ToSlash(relative))
		if err != nil {
			return err
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(entry, input)
		closeErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		_ = archive.Close()
		return err
	}
	return archive.Close()
}
