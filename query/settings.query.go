package query

import (
	"github.com/aidenappl/openbucket-api/db"
)

func GetSetting(engine db.Queryable, key string) (string, error) {
	var value string
	err := engine.QueryRow("SELECT value FROM settings WHERE `key` = ?", key).Scan(&value)
	return value, err
}

func SetSetting(engine db.Queryable, key, value string) error {
	_, err := engine.Exec(
		"INSERT INTO settings (`key`, value) VALUES (?, ?) ON DUPLICATE KEY UPDATE value = VALUES(value)",
		key, value,
	)
	return err
}

func DeleteSetting(engine db.Queryable, key string) error {
	_, err := engine.Exec("DELETE FROM settings WHERE `key` = ?", key)
	return err
}

func GetSettingsByPrefix(engine db.Queryable, prefix string) (map[string]string, error) {
	rows, err := engine.Query("SELECT `key`, value FROM settings WHERE `key` LIKE ?", prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		result[k] = v
	}
	return result, rows.Err()
}

// DeleteSettingExisted deletes a key and reports whether a row was removed.
//
// ⚠️ THIS MAKES SSO STATE CONSUMPTION ATOMIC. The old ValidateState did GetSetting
// then an unconditional DeleteSetting and returned a bool — DeleteSetting reports
// success whether or not a row was there, so two concurrent callbacks presenting
// the same state BOTH passed. That is the window an attacker replaying a captured
// callback URL alongside the real one is aiming at.
//
// RowsAffected plus MariaDB's row lock means exactly one of N concurrent callers
// sees true. Winning the DELETE — not having read the row — authorises use of the
// record. Do not reduce this to a SELECT followed by DeleteSetting.
func DeleteSettingExisted(engine db.Queryable, key string) (bool, error) {
	res, err := engine.Exec("DELETE FROM settings WHERE `key` = ?", key)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
