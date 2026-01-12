import http from 'k6/http';
import { check, sleep } from 'k6';

// ตั้งค่าการทดสอบ
export const options = {
  // จำลองคน 100 คน (Virtual Users)
  vus: 100, 
  // ยิงถล่มเป็นเวลา 10 วินาที
  duration: '10s', 
};

export default function () {
  const url = 'http://localhost/api/buy';
  
  // สุ่ม User ID ไม่ให้ซ้ำกันมาก (1 - 100,000)
  const userId = Math.floor(Math.random() * 100000) + 1;

  const payload = JSON.stringify({
    user_id: userId,
    product_id: 1, // ซื้อสินค้า ID 1
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  // ยิง Request!
  const res = http.post(url, payload, params);

  // เช็คผลลัพธ์ (Check)
  // เราคาดหวัง 202 (Accepted) หรือ 400 (Sold Out)
  // แต่ต้องไม่ไช่ 500 (Internal Server Error)
  check(res, {
    'is status 202 or 400': (r) => r.status === 202 || r.status === 400,
    'is not 500': (r) => r.status !== 500,
  });

  sleep(0.1); // พัก 0.1 วิ ก่อนยิงใหม่ (จำลองคนกดย้ำๆ)
}