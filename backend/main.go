package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

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
	TrustScore         int  `json:"trust_score"`
	OrientationClaimed bool `json:"orientation_claimed"`
	LastClaimPeriod string `json:"last_claim_period"`
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
const (
	NISTContractAddress = "0xEC08895F6C21f17b9b4D32b5bE3CcAA79b0E58ED"
	ReceiverAddress = "0x82F4F791A79f0Dc6a8eE39888c05D0B4E9A093Cb"
	BSCChainID = int64(97)
	ClaimAmountNIST = int64(200)
)

func currentClaimPeriod() string {
	now := time.Now().UTC()
	period := (int(now.Month()) - 1) / 2
	return fmt.Sprintf("%d-%02d", now.Year(), period)
}

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

// แจก 200 NIST + 5 Trust Score ให้กระเป๋าละ 1 ครั้งต่อรอบ 2 เดือน
func claimNISTHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed); return }
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
	wallet := req["wallet_address"]
	if !isHexAddress(wallet) { http.Error(w, "wallet_address is invalid", http.StatusBadRequest); return }
	mutex.Lock(); defer mutex.Unlock()
	period := currentClaimPeriod()
	for i := range users {
		if !strings.EqualFold(users[i].WalletAddress, wallet) { continue }
		if users[i].LastClaimPeriod == period {
			w.Header().Set("Content-Type", "application/json"); w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]interface{}{"message":"Claim already used for this 2-month period", "claimed":true, "period":period}); return
		}
		txHash, err := sendNISTTransfer(wallet, ClaimAmountNIST)
		if err != nil {
			log.Printf("❌ Claim failed for wallet %s: %v", wallet, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("✅ Claim transaction submitted for wallet %s: %s", wallet, txHash)
		users[i].LastClaimPeriod = period
		users[i].TrustScore += 5
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"message":"Claim successful", "amount":ClaimAmountNIST, "new_score":users[i].TrustScore, "tx_hash":txHash, "period":period, "claimed":true}); return
	}
	http.Error(w, "User not found", http.StatusNotFound)
}
// รับคะแนนปฐมนิเทศได้เพียง 1 ครั้งต่อกระเป๋า (+5 Trust Score)
func claimOrientationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed); return }
	var req struct { WalletAddress string `json:"wallet_address"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
	if req.WalletAddress == "" { http.Error(w, "wallet_address is required", http.StatusBadRequest); return }
	mutex.Lock(); defer mutex.Unlock()
	for i := range users {
		if users[i].WalletAddress != req.WalletAddress { continue }
		if users[i].OrientationClaimed { http.Error(w, "Orientation score already claimed", http.StatusConflict); return }
		users[i].TrustScore += 5; users[i].OrientationClaimed = true
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"message":"Orientation score claimed successfully", "new_score":users[i].TrustScore, "orientation_claimed":true})
		return
	}
	http.Error(w, "User not found", http.StatusNotFound)
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
	http.HandleFunc("/api/claim", enableCORS(claimNISTHandler))
	http.HandleFunc("/api/orientation-claim", enableCORS(claimOrientationHandler))
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
