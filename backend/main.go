package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// ==========================================
// 1. โครงสร้างข้อมูล (Structs)
// ==========================================
type User struct {
	WalletAddress string `json:"wallet_address"`
	TrustScore    int    `json:"trust_score"`
}

type Booking struct {
	ID            string `json:"id"`
	WalletAddress string `json:"wallet_address"`
	RoomName      string `json:"room_name"`
	Date          string `json:"date"`
	Time          string `json:"time"`
	Status        string `json:"status"` // "Confirmed" หรือ "Cancelled"
	Amount        int    `json:"amount"`
	TxHash        string `json:"tx_hash"`
}

type Transaction struct {
	ID            string `json:"id"`
	WalletAddress string `json:"wallet_address"`
	To            string `json:"to"`
	Amount        int    `json:"amount"`
	TxHash        string `json:"tx_hash"`
	Type          string `json:"type"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
}

// ==========================================
// 2. ฐานข้อมูลแบบ In-Memory Slice
// ==========================================
var users []User
var bookings []Booking
var transactions []Transaction
var mutex sync.Mutex // ใช้ Mutex เพื่อป้องกันปัญหาตอนมีคนเรียก API พร้อมกัน

// ==========================================
// 3. Middleware สำหรับจัดการ CORS (ให้หน้าเว็บ :5500 เรียก API ได้)
// ==========================================
func enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

// ==========================================
// 4. API Handlers
// ==========================================

// ดึงข้อมูลผู้ใช้ (ถ้าไม่มี ให้สร้างใหม่ เริ่มต้น 80 คะแนน)
func getUserHandler(w http.ResponseWriter, r *http.Request) {
	wallet := r.URL.Query().Get("wallet")
	if wallet == "" {
		http.Error(w, "Missing wallet address", http.StatusBadRequest)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	// ค้นหาใน Slice
	for i := range users {
		if users[i].WalletAddress == wallet {
			json.NewEncoder(w).Encode(users[i])
			return
		}
	}

	// ถ้าไม่เจอ ให้สร้าง User ใหม่ (Trust Score เริ่มที่ 80)
	newUser := User{WalletAddress: wallet, TrustScore: 80}
	users = append(users, newUser)

	json.NewEncoder(w).Encode(newUser)
}

// ดึงประวัติการจองทั้งหมดของกระเป๋านั้น
func getBookingsHandler(w http.ResponseWriter, r *http.Request) {
	wallet := r.URL.Query().Get("wallet")

	mutex.Lock()
	defer mutex.Unlock()

	var userBookings []Booking
	for _, b := range bookings {
		if b.WalletAddress == wallet {
			userBookings = append(userBookings, b)
		}
	}

	// ถ้าไม่มีเลย ให้ส่ง Array เปล่ากลับไปแทนที่จะเป็น null
	if userBookings == nil {
		userBookings = []Booking{}
	}

	json.NewEncoder(w).Encode(userBookings)
}

// สร้างการจองใหม่
func bookRoomHandler(w http.ResponseWriter, r *http.Request) {
	var newBooking Booking
	if err := json.NewDecoder(r.Body).Decode(&newBooking); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	mutex.Lock()
	// จำลองสร้าง ID ด้วยเวลา
	newBooking.ID = fmt.Sprintf("BOK-%d", time.Now().Unix())
	newBooking.Status = "Confirmed"
	bookings = append(bookings, newBooking)
	mutex.Unlock()

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newBooking)
}

// ยกเลิกการจอง (เปลี่ยนสถานะ และหัก Trust Score 2 คะแนน)
func cancelBookingHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BookingID     string `json:"booking_id"`
		WalletAddress string `json:"wallet_address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	// 1. หาการจองและเปลี่ยนสถานะ
	foundBooking := false
	for i := range bookings {
		if bookings[i].ID == req.BookingID && bookings[i].WalletAddress == req.WalletAddress {
			bookings[i].Status = "Cancelled"
			foundBooking = true
			break
		}
	}

	if !foundBooking {
		http.Error(w, "Booking not found", http.StatusNotFound)
		return
	}

	// 2. หาผู้ใช้และหักคะแนน Trust Score
	for i := range users {
		if users[i].WalletAddress == req.WalletAddress {
			users[i].TrustScore -= 2
			break
		}
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "Cancelled successfully and Trust Score updated"})
}

// ทำกิจกรรมรับคะแนน (เพิ่ม Trust Score 5 คะแนน)
func earnActivityHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WalletAddress string `json:"wallet_address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	for i := range users {
		if users[i].WalletAddress == req.WalletAddress {
			users[i].TrustScore += 5
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message":   "Activity completed",
				"new_score": users[i].TrustScore,
			})
			return
		}
	}

	http.Error(w, "User not found", http.StatusNotFound)
}


// บันทึก Transaction หลังจาก Frontend โอน NIST สำเร็จบน Blockchain
func createTransactionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var tx Transaction
	if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if tx.WalletAddress == "" || tx.TxHash == "" {
		http.Error(w, "wallet_address and tx_hash are required", http.StatusBadRequest)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	tx.ID = fmt.Sprintf("TX-%d", time.Now().UnixNano())
	tx.Status = "Confirmed"
	tx.CreatedAt = time.Now().Format(time.RFC3339)

	transactions = append(transactions, tx)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(tx)
}

// ดึงประวัติ Transaction ของกระเป๋า
func getTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	wallet := r.URL.Query().Get("wallet")
	if wallet == "" {
		http.Error(w, "Missing wallet address", http.StatusBadRequest)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	result := []Transaction{}
	for _, tx := range transactions {
		if tx.WalletAddress == wallet {
			result = append(result, tx)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"service": "NIST-Campus API",
	})
}

// ==========================================
// 5. ฟังก์ชันหลัก (Main)
// ==========================================
func main() {
	http.HandleFunc("/", enableCORS(healthHandler))
	http.HandleFunc("/api/user", enableCORS(getUserHandler))
	http.HandleFunc("/api/bookings", enableCORS(getBookingsHandler))
	http.HandleFunc("/api/book", enableCORS(bookRoomHandler))
	http.HandleFunc("/api/cancel", enableCORS(cancelBookingHandler))
	http.HandleFunc("/api/earn", enableCORS(earnActivityHandler))
	http.HandleFunc("/api/transaction", enableCORS(createTransactionHandler))
	http.HandleFunc("/api/transactions", enableCORS(getTransactionsHandler))

	port := 8080
	if envPort := os.Getenv("PORT"); envPort != "" {
		if parsed, err := strconv.Atoi(envPort); err == nil {
			port = parsed
		}
	}

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("🚀 NIST-Campus Backend running on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
