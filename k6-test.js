import http from 'k6/http';
import { check, sleep } from 'k6';

// ===================================================================
// ID Produk Anda dari Postman sudah dimasukkan di sini
// ===================================================================
const VALID_PRODUCT_ID = '780e2e4c-9cf2-47f6-bfc6-ff46df8bcb13';
// ===================================================================

// Opsi eksekusi tes
export const options = {
  stages: [
    { duration: '15s', target: 500 }, // Naik ke 500 VUs (Virtual Users) dalam 15 detik
    { duration: '30s', target: 500 }, // Tahan di 500 VUs selama 30 detik
    { duration: '15s', target: 1000 }, // Naik ke 1000 VUs dalam 15 detik
    { duration: '30s', target: 1000 }, // Tahan di 1000 VUs selama 30 detik (Tes utama)
    { duration: '10s', target: 0 },    // Turun kembali ke 0
  ],
  thresholds: {
    // --- INI ADALAH PERBAIKANNYA ---
    // Sintaks yang benar adalah 'metric{tag_name:tag_value}'
    'checks{check:status_is_201}': ['rate>0.95'], 
    
    // Target utama: http_reqs (request per detik) harus > 1000
    'http_reqs': ['rate>=1000'],
  },
};

// Fungsi utama yang dijalankan oleh setiap Virtual User (VU)
export default function () {
  const url = 'http://localhost:8080/api/v1/orders';
  
  const payload = JSON.stringify({
    productId: VALID_PRODUCT_ID,
    quantity: 1, // Pesan 1 unit
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  // Kirim request POST
  const res = http.post(url, payload, params);

  // Verifikasi respons
  check(res, {
    // Nama 'check' ini 'status_is_201', yang akan difilter di threshold
    'status_is_201': (r) => r.status === 201,
  });
}