package db

import "database/sql"

type DeviceInfo struct {
	Type        string `json:"device_type"`
	DeviceID    string `json:"device_id"`
	IP          string `json:"device_ip"`
	Port        int    `json:"device_port"`
	Status      string `json:"device_status"`
	UUID        string `json:"device_uuid"`
	DisplayName string `json:"display_name"`
}

func QueryDeviceTypeByUUID(db *sql.DB, deviceUUID string) (DeviceInfo, error) {
	var device DeviceInfo
	err := db.QueryRow("SELECT type, id_adb, uuid, display_name, ip, port, status_adb, status_ui FROM devices WHERE uuid = ?", deviceUUID).Scan(&device.Type, &device.DeviceID, &device.UUID, &device.DisplayName, &device.IP, &device.Port, &device.Status, &device.Status)
	if err != nil {
		return DeviceInfo{}, err
	}
	return device, nil
}

func QueryDeviceTypeByID(db *sql.DB, deviceID int) (string, error) {
	var deviceType string
	err := db.QueryRow("SELECT type FROM devices WHERE id = ?", deviceID).Scan(&deviceType)
	if err != nil {
		return "", err
	}
	return deviceType, nil
}

func DeviceExists(db *sql.DB, dtype string, ip string, port int) (bool, error) {
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM devices WHERE type = ? AND ip = ? AND port = ?)", dtype, ip, port).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func InsertDevice(db *sql.DB, uuid string, dtype string, idAdb string, displayName string, ip string, port int, statusADB string, statusUI string) error {
	_, err := db.Exec("INSERT INTO devices (uuid, type, id_adb, display_name, ip, port, status_adb, status_ui) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", uuid, dtype, idAdb, displayName, ip, port, statusADB, statusUI)
	return err
}

func QueryAllDevices(db *sql.DB) ([]DeviceInfo, error) {
	rows, err := db.Query("SELECT type, id_adb, uuid, display_name, ip, port, status_adb, status_ui FROM devices")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []DeviceInfo
	for rows.Next() {
		var device DeviceInfo
		err := rows.Scan(&device.Type, &device.DeviceID, &device.UUID, &device.DisplayName, &device.IP, &device.Port, &device.Status, &device.Status)
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return devices, nil
}

func UpdateDeviceByUUID(db *sql.DB, uuid string, dtype string, idAdb string, displayName string, ip string, port int, statusADB string, statusUI string) error {
	_, err := db.Exec("UPDATE devices SET type = ?, id_adb = ?, display_name = ?, ip = ?, port = ?, status_adb = ?, status_ui = ? WHERE uuid = ?", dtype, idAdb, displayName, ip, port, statusADB, statusUI, uuid)
	return err
}

func UpdateDeviceByTypeIPPort(db *sql.DB, dtype string, ip string, port int, idAdb string, displayName string, statusADB string, statusUI string) error {
	_, err := db.Exec("UPDATE devices SET id_adb = ?, display_name = ?, status_adb = ?, status_ui = ? WHERE type = ? AND ip = ? AND port = ?", idAdb, displayName, statusADB, statusUI, dtype, ip, port)
	return err
}

func UpdateDeviceDisplayNameByUUID(db *sql.DB, deviceUUID string, displayName string) error {
	_, err := db.Exec("UPDATE devices SET display_name = ? WHERE uuid = ?", displayName, deviceUUID)
	return err
}
