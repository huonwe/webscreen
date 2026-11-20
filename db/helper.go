package db

import "database/sql"

func DeviceTypeByUUID(db *sql.DB, deviceUUID string) (string, error) {
	var deviceType string
	err := db.QueryRow("SELECT device_type FROM devices WHERE uuid = ?", deviceUUID).Scan(&deviceType)
	if err != nil {
		return "", err
	}
	return deviceType, nil
}

func DeviceTypeByID(db *sql.DB, deviceID int) (string, error) {
	var deviceType string
	err := db.QueryRow("SELECT device_type FROM devices WHERE id = ?", deviceID).Scan(&deviceType)
	if err != nil {
		return "", err
	}
	return deviceType, nil
}
