package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// สร้าง Struct สำหรับห้อง
type Resource struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Building string `json:"building"`
	Capacity int    `json:"capacity"`
	Price    int    `json:"price"`
	Status   string `json:"status"`
}

// ใช้ In-Memory Slice เก็บข้อมูลชั่วคราว
var resources = []Resource{
	{ID: "A401", Name: "ห้องศึกษาค้นคว้ากลุ่ม 1", Building: "อาคารวิทยบริการ A", Capacity: 8, Price: 50, Status: "Available"},
	{ID: "B203", Name: "Smart Classroom", Building: "อาคารวิทยบริการ B", Capacity: 15, Price: 30, Status: "Available"},
}

// ฟังก์ชันสำหรับส่งข้อมูลห้อง
func getResources(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // ป้องกัน CORS Error
	json.NewEncoder(w).Encode(resources)
}

func main() {
	http.HandleFunc("/api/resources", getResources)

	fmt.Println("🚀 Backend API running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println("Server failed:", err)
	}
}
