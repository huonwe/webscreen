package webservice

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type UnlockAttemptRecord struct {
	Attempts  int
	IsLocked  bool
	LockUntil time.Time
}

// User need to enter correct PIN to unlock the web access
func (wm *WebMaster) handleUnlock(c *gin.Context) {
	var req struct {
		PIN string `json:"pin"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"result": "error", "message": "Invalid request"})
		return
	}
	sIP := c.ClientIP()
	var record UnlockAttemptRecord
	record, ok := wm.UnlockAttemptRecords[sIP]
	if ok {
		if record.IsLocked {
			if time.Now().Before(record.LockUntil) {
				c.JSON(200, gin.H{"result": "failed", "message": "Too many attempts, please try again later", "leftTries": 0, "lockUntil": record.LockUntil})
				return
			} else {
				// Unlock the record
				record.IsLocked = false
				record.Attempts = 0
				wm.UnlockAttemptRecords[sIP] = record
			}
		}
		if record.Attempts >= 5 {
			record.IsLocked = true
			record.LockUntil = time.Now().Add(10 * time.Minute)
			wm.UnlockAttemptRecords[sIP] = record

			c.JSON(200, gin.H{"result": "failed", "message": "Too many attempts, please try again later", "leftTries": 0, "lockUntil": record.LockUntil})
			return
		}
	} else {
		record = UnlockAttemptRecord{
			Attempts:  0,
			IsLocked:  false,
			LockUntil: time.Time{},
		}
		wm.UnlockAttemptRecords[sIP] = record
	}

	if req.PIN != wm.pin {
		c.JSON(200, gin.H{"result": "failed", "message": "Incorrect PIN", "leftTries": 5 - record.Attempts})
		record.Attempts++
		wm.UnlockAttemptRecords[sIP] = record
		return
	}

	token, err := wm.GenerateToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token 生成失败"})
		return
	}
	c.SetCookie("auth_token", token, 3600*2, "/", "", false, true)
	c.JSON(200, gin.H{"result": "success", "message": "Unlocked", "token": token})
	delete(wm.UnlockAttemptRecords, sIP)
}

// Set a new PIN for web access
// func (wm *WebMaster) handleSetPIN(c *gin.Context) {
// 	var req struct {
// 		NewPIN string `json:"new_pin"`
// 	}
// 	if err := c.BindJSON(&req); err != nil {
// 		c.JSON(400, gin.H{"error": "Invalid request"})
// 		return
// 	}
// }
