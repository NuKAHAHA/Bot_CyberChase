package models

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

type PGInterval time.Duration

func (d *PGInterval) Scan(value interface{}) error {
	var str string
	switch v := value.(type) {
	case string:
		str = v
	case []byte:
		str = string(v)
	default:
		return fmt.Errorf("cannot scan %T into PGInterval", value)
	}

	// Если строка пустая или "00:00:00", то устанавливаем нулевую длительность
	if str == "" || str == "00:00:00" {
		*d = PGInterval(0)
		return nil
	}

	// Обработка формата PostgreSQL interval
	// Может быть несколько форматов:
	// 1. "01:02:03" → 1h2m3s
	// 2. "X seconds" или "X microseconds" и т.д.

	if strings.Contains(str, ":") {
		// Формат "01:02:03"
		parts := strings.Split(str, ":")
		if len(parts) != 3 {
			return fmt.Errorf("invalid interval format: %s", str)
		}
		h, m, s := parts[0], parts[1], parts[2]
		parsed, err := time.ParseDuration(fmt.Sprintf("%sh%sm%ss", h, m, s))
		if err != nil {
			return err
		}
		*d = PGInterval(parsed)
		return nil
	} else if strings.Contains(str, "seconds") {
		// Формат "X seconds"
		var seconds int64
		_, err := fmt.Sscanf(str, "%d seconds", &seconds)
		if err != nil {
			return fmt.Errorf("invalid seconds format: %s, %v", str, err)
		}
		*d = PGInterval(time.Duration(seconds) * time.Second)
		return nil
	}

	// Если не распознали формат, возвращаем ошибку
	return fmt.Errorf("unrecognized interval format: %s", str)
}

func (d PGInterval) Value() (driver.Value, error) {
	seconds := int64(time.Duration(d).Seconds())
	return fmt.Sprintf("%d seconds", seconds), nil
}

// Метод для удобства доступа к duration
func (d PGInterval) Duration() time.Duration {
	return time.Duration(d)
}

// Строковое представление для отображения
func (d PGInterval) String() string {
	duration := time.Duration(d)
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}
