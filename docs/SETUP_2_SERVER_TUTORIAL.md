# Tutorial Setup 2 Server untuk VPS NAT Backend

Dokumen ini menjelaskan setup dari nol sampai backend `vps-nat` siap dipakai dengan arsitektur sederhana `2 server`.

Arsitektur yang dipakai:
- `Server 1` menjalankan backend Go + PostgreSQL
- `Server 2` menjalankan Incus + Caddy

Model ini cocok untuk MVP karena:
- lebih hemat biaya dan lebih mudah dikelola
- backend dan database berada di satu tempat
- node Incus fokus untuk provisioning container customer
- Caddy ditempatkan dekat Incus agar reverse proxy ke IP private container lebih sederhana

## 1. Gambaran Arsitektur

Alur sederhananya seperti ini:

1. User berinteraksi dengan bot Telegram.
2. Bot memanggil endpoint backend `vps-nat`.
3. Backend menyimpan data ke PostgreSQL di `Server 1`.
4. Backend mengakses API Incus di `Server 2` untuk create, start, stop, reinstall, dan operasi lain.
5. Backend mengakses Caddy Admin API di `Server 2` untuk setup domain reverse proxy.
6. Container customer berjalan di dalam Incus dan diakses lewat NAT port mapping atau domain.

Pembagian peran server:

### Server 1
- Go backend `vps-nat`
- PostgreSQL
- webhook payment
- admin API
- endpoint internal untuk bot Telegram

### Server 2
- Incus
- bridge network Incus
- NAT / network forward
- Caddy
- endpoint Caddy Admin API
- container customer

## 2. Kebutuhan Awal

Minimal siapkan:
- `2 VPS / server`
- `1 domain` untuk backend API, misalnya `api.example.com`
- opsional `1 domain / wildcard domain` untuk layanan customer
- `1 bot Telegram` untuk customer
- opsional `1 bot Telegram` khusus alert admin
- akun Pakasir jika ingin memakai QRIS
- akses root atau sudo ke kedua server

Rekomendasi spesifikasi awal:

### Server 1: App + PostgreSQL
- 2 vCPU
- 4 GB RAM
- 40-80 GB SSD
- Ubuntu 24.04 LTS
- IP publik

### Server 2: Incus + Caddy
- 4 vCPU atau lebih
- 8 GB RAM atau lebih
- 80 GB SSD atau lebih
- Ubuntu 24.04 LTS
- IP publik

Catatan:
- `Server 2` sebaiknya lebih kuat karena akan menjalankan container customer.
- Untuk target MVP kecil sekitar 15 user, pembagian ini masih masuk akal.

## 3. Topologi Jaringan yang Disarankan

Gunakan asumsi:
- `Server 1` IP publik: `203.0.113.10`
- `Server 2` IP publik: `203.0.113.20`
- bridge Incus internal: `10.10.10.1/24`
- container customer mendapat IP private dari bridge tersebut

Port yang perlu diperhatikan:

### Server 1
- `80/tcp` dan `443/tcp` bila backend nanti dipasang di balik reverse proxy
- `8080/tcp` untuk proses API jika expose langsung
- `5432/tcp` PostgreSQL, idealnya jangan dibuka ke publik

### Server 2
- `8443/tcp` Incus remote API, batasi hanya agar bisa diakses dari `Server 1`
- `2019/tcp` Caddy Admin API, batasi hanya agar bisa diakses dari `Server 1`
- `80/tcp` dan `443/tcp` untuk trafik domain customer
- port NAT customer sesuai kebutuhan paket

## 4. Software yang Diinstall per Server

### Server 1
Install:
- `golang`
- `git`
- `postgresql`
- `curl`
- `ufw` atau firewall lain
- opsional `caddy` atau `nginx` jika API backend ingin diberi reverse proxy publik
- opsional `make`, `jq`, `psql` client tools

Tidak perlu install di Server 1:
- Incus
- Caddy untuk domain customer

### Server 2
Install:
- `incus`
- `caddy`
- `curl`
- `ufw` atau firewall lain

Tidak perlu install di Server 2:
- Golang untuk menjalankan backend
- PostgreSQL untuk database utama

## 5. Setup Server 1: Backend + PostgreSQL

## 5.1 Update sistem

```bash
sudo apt update
sudo apt upgrade -y
```

## 5.2 Install paket dasar

```bash
sudo apt install -y git curl unzip postgresql postgresql-contrib
```

Install Go sesuai versi yang dipakai repo. Saat dokumen ini ditulis, [`go.mod`](/root/project/vps-nat/go.mod) memakai:

```text
go 1.26.1
```

Kalau versi itu belum tersedia di image server kamu, pakai versi Go stabil terdekat yang kompatibel lalu sesuaikan saat build.

Contoh cek versi:

```bash
go version
```

## 5.3 Buat database PostgreSQL

Masuk ke PostgreSQL:

```bash
sudo -u postgres psql
```

Lalu buat database dan user:

```sql
CREATE DATABASE vps_nat;
CREATE USER vps_nat_user WITH ENCRYPTED PASSWORD 'ganti-password-kuat';
GRANT ALL PRIVILEGES ON DATABASE vps_nat TO vps_nat_user;
\q
```

Karena migration pertama mengaktifkan `pgcrypto`, pastikan extension dapat dibuat oleh role yang dipakai migration. Kalau perlu, jalankan migration dengan user yang punya privilege lebih tinggi.

## 5.4 Clone project

```bash
git clone https://github.com/DioSaputra28/vps-nat.git
cd vps-nat
```

## 5.5 Siapkan environment file

```bash
cp .env.example .env
```

Isi minimal `.env` untuk arsitektur 2 server:

```env
APP_NAME=vps-nat
APP_ENV=production

HTTP_HOST=0.0.0.0
HTTP_PORT=8080

AUTH_ADMIN_SESSION_TTL=24h
TELEGRAM_BOT_SECRET=isi-secret-yang-sama-dengan-bot

PAKASIR_BASE_URL=https://app.pakasir.com
PAKASIR_PROJECT_SLUG=
PAKASIR_API_KEY=

ADMIN_ALERT_TELEGRAM_BOT_TOKEN=
ADMIN_ALERT_TELEGRAM_CHAT_ID=

CADDY_ADMIN_URL=http://203.0.113.20:2019
CADDY_ADMIN_API_TOKEN=isi-token-caddy

DB_HOST=127.0.0.1
DB_PORT=5432
DB_USER=vps_nat_user
DB_PASSWORD=ganti-password-kuat
DB_NAME=vps_nat
DB_SSLMODE=disable
DB_TIMEZONE=UTC
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=10
DB_CONN_MAX_LIFETIME=30m
DB_CONN_MAX_IDLE_TIME=5m

INCUS_ENABLED=true
INCUS_MODE=remote
INCUS_REMOTE_ADDR=https://203.0.113.20:8443
INCUS_NETWORK_NAME=incusbr0
INCUS_USER_AGENT=vps-nat-backend
INCUS_TLS_CLIENT_CERT_PATH=/opt/vps-nat/certs/incus-client.crt
INCUS_TLS_CLIENT_KEY_PATH=/opt/vps-nat/certs/incus-client.key
INCUS_TLS_CA_PATH=/opt/vps-nat/certs/incus-ca.crt
INCUS_TLS_SERVER_CERT_PATH=/opt/vps-nat/certs/incus-server.crt
INCUS_TLS_INSECURE_SKIP_VERIFY=false
```

Penjelasan penting:
- `INCUS_MODE=remote` karena backend dan Incus ada di server berbeda
- `INCUS_REMOTE_ADDR` mengarah ke `Server 2`
- `CADDY_ADMIN_URL` juga mengarah ke `Server 2`
- sertifikat TLS Incus disimpan di `Server 1` karena backend yang akan menjadi client API

## 5.6 Apply migration database

Repo ini menyimpan SQL migration di folder [`database`](/root/project/vps-nat/database), tetapi belum menyediakan migration runner bawaan.

Urutan file saat ini:
- `000001_enable_pgcrypto.up.sql`
- `000002_create_users.up.sql`
- `000003_create_admin_users.up.sql`
- `000004_create_wallets.up.sql`
- `000005_create_packages.up.sql`
- `000006_create_orders.up.sql`
- `000007_create_wallet_topups.up.sql`
- `000008_create_payments.up.sql`
- `000009_create_invoices.up.sql`
- `000010_create_nodes.up.sql`
- `000011_create_services.up.sql`
- `000012_add_orders_target_service_fk.up.sql`
- `000013_create_service_instances.up.sql`
- `000014_create_service_port_mappings.up.sql`
- `000015_create_service_domains.up.sql`
- `000016_create_provisioning_jobs.up.sql`
- `000017_create_service_transfers.up.sql`
- `000018_create_service_events.up.sql`
- `000019_create_wallet_transactions.up.sql`
- `000020_create_activity_logs.up.sql`
- `000021_create_resource_alerts.up.sql`
- `000022_create_server_costs.up.sql`
- `000023_create_support_tickets.up.sql`
- `000024_create_support_ticket_messages.up.sql`
- `000025_create_admin_sessions.up.sql`
- `000026_normalize_suspended_statuses.up.sql`

Contoh apply manual dengan `psql`:

```bash
for f in $(ls database/*.up.sql | sort); do
  echo "applying $f"
  PGPASSWORD='ganti-password-kuat' psql \
    -h 127.0.0.1 \
    -U vps_nat_user \
    -d vps_nat \
    -f "$f" || break
done
```

Kalau migration `000001` gagal karena privilege extension, jalankan migration awal memakai role PostgreSQL yang lebih tinggi.

## 5.7 Seed admin pertama

Gunakan command bawaan repo:

```bash
go run ./cmd/admin-seed \
  --email admin@example.com \
  --password 'AdminPass123!' \
  --role super_admin
```

Role yang didukung:
- `super_admin`
- `admin`

## 5.8 Jalankan backend

Untuk uji awal:

```bash
go run ./cmd/api
```

Health endpoint:
- `GET /health`
- `GET /healthz`

Kalau ingin production yang lebih rapi, build binary lalu jalankan dengan `systemd`.

Contoh build:

```bash
go build -o bin/vps-nat-api ./cmd/api
```

## 5.9 Buat user dan direktori aplikasi

Supaya lebih rapi untuk production, pindahkan hasil deploy ke direktori tetap, misalnya:

```bash
sudo mkdir -p /opt/vps-nat
sudo mkdir -p /opt/vps-nat/bin
sudo mkdir -p /opt/vps-nat/current
sudo mkdir -p /opt/vps-nat/certs
```

Opsional buat user service khusus:

```bash
sudo useradd --system --home /opt/vps-nat --shell /usr/sbin/nologin vpsnat
sudo chown -R vpsnat:vpsnat /opt/vps-nat
```

Lalu salin binary dan `.env` ke lokasi deployment:

```bash
cp bin/vps-nat-api /opt/vps-nat/bin/
cp .env /opt/vps-nat/current/.env
```

Pastikan file sertifikat Incus di `/opt/vps-nat/certs` juga bisa dibaca oleh user service.

## 5.10 Buat service systemd untuk backend

Buat file:

```text
/etc/systemd/system/vps-nat.service
```

Isi contoh:

```ini
[Unit]
Description=VPS NAT Backend API
After=network.target postgresql.service
Wants=postgresql.service

[Service]
Type=simple
User=vpsnat
Group=vpsnat
WorkingDirectory=/opt/vps-nat/current
EnvironmentFile=/opt/vps-nat/current/.env
ExecStart=/opt/vps-nat/bin/vps-nat-api
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Lalu aktifkan:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now vps-nat
sudo systemctl status vps-nat
```

Untuk melihat log:

```bash
journalctl -u vps-nat -f
```

## 5.11 Firewall dasar di Server 1

Contoh `ufw` sederhana:

```bash
sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 8080/tcp
sudo ufw enable
```

Catatan:
- kalau backend akan selalu diakses lewat reverse proxy lokal, port `8080` tidak perlu dibuka ke publik
- PostgreSQL `5432` sebaiknya tidak dibuka ke internet

## 5.12 Reverse proxy backend API di Server 1

Kalau mau endpoint backend lebih rapi, pasang Caddy di `Server 1` untuk domain API.

Contoh install:

```bash
sudo apt install -y caddy
```

Contoh file:

```text
/etc/caddy/Caddyfile
```

Isi sederhana:

```caddy
api.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

Lalu reload:

```bash
sudo systemctl reload caddy
sudo systemctl status caddy
```

Dengan model ini:
- domain `api.example.com` mengarah ke `Server 1`
- Caddy di `Server 1` menangani TLS untuk backend API
- proses Go tetap listen di `127.0.0.1:8080` atau `0.0.0.0:8080`

## 6. Setup Server 2: Incus + Caddy

## 6.1 Update sistem

```bash
sudo apt update
sudo apt upgrade -y
```

## 6.2 Install Incus

Metode install tergantung distro. Untuk Ubuntu modern, biasanya Incus dipasang lewat paket atau snap sesuai kebijakan operasional kamu. Yang penting hasil akhirnya:
- service Incus aktif
- socket atau remote API Incus aktif
- bridge network tersedia
- server bisa membuat container

Setelah install, lakukan inisialisasi:

```bash
sudo incus admin init
```

Saat inisialisasi, fokus pada:
- membuat storage pool
- membuat bridge network, misalnya `incusbr0`
- menyiapkan range IP internal untuk container

Sesudah selesai, cek:

```bash
incus network list
incus profile list
incus storage list
```

Pastikan nama network sama dengan nilai `INCUS_NETWORK_NAME` pada `.env`.

## 6.3 Aktifkan remote API Incus

Karena backend berjalan di `Server 1`, Incus harus bisa diakses secara remote dari sana.

Prinsip penting:
- jangan buka API Incus ke seluruh internet
- hanya izinkan akses dari IP `Server 1`
- gunakan TLS certificate

Port default remote API:

```text
8443/tcp
```

Setelah remote API aktif, ambil atau siapkan:
- CA certificate
- client certificate
- client key
- server certificate bila diperlukan validasi eksplisit

Lalu salin certificate client yang dibutuhkan backend ke `Server 1`, misalnya ke:

```text
/opt/vps-nat/certs/
```

File yang perlu tersedia di `Server 1`:
- `incus-client.crt`
- `incus-client.key`
- `incus-ca.crt`
- `incus-server.crt`

Catatan:
- Repo ini membaca file-file itu dari path env `INCUS_TLS_*`
- backend akan gagal terkoneksi jika path salah atau sertifikat tidak cocok

## 6.4 Siapkan image dan uji provisioning manual

Sebelum backend dihubungkan, uji dulu bahwa Incus sehat.

Contoh:

```bash
incus image list images:
incus launch images:ubuntu/24.04 test-vm
incus list
incus delete test-vm --force
```

Kalau langkah ini belum berhasil, jangan lanjut ke backend dulu.

## 6.5 Siapkan NAT / network forward

Project ini memakai model VPS NAT:
- container mendapat IP private
- akses publik diberikan lewat forwarded port pada node

Karena itu, di `Server 2` kamu perlu memastikan:
- bridge network Incus berjalan normal
- network forward atau port mapping Incus bisa dibuat
- firewall tidak memblokir port customer yang memang ingin dibuka

Ini penting karena backend akan membuat mapping port saat provisioning service customer.

## 6.6 Install Caddy

Install Caddy di `Server 2` lalu pastikan service aktif.

Yang dibutuhkan backend dari Caddy:
- Caddy berjalan di node yang bisa menjangkau IP private container
- Admin API aktif
- ada token untuk autentikasi API jika kamu mengamankannya

Port penting:
- `80/tcp`
- `443/tcp`
- `2019/tcp` untuk Admin API

`2019/tcp` sebaiknya hanya bisa diakses dari `Server 1`.

## 6.7 Konfigurasi Caddy untuk mode Admin API

Backend ini tidak menulis file config Caddy secara manual. Backend mengirim konfigurasi reverse proxy melalui Caddy Admin API.

Artinya:
- Caddy harus aktif
- Admin API harus dapat diakses dari `Server 1`
- token yang dipakai backend harus valid

Di `.env` backend, bagian yang dipakai adalah:

```env
CADDY_ADMIN_URL=http://203.0.113.20:2019
CADDY_ADMIN_API_TOKEN=isi-token-caddy
```

## 6.8 Firewall dasar di Server 2

Contoh `ufw`:

```bash
sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow from 203.0.113.10 to any port 8443 proto tcp
sudo ufw allow from 203.0.113.10 to any port 2019 proto tcp
sudo ufw enable
```

Dengan aturan ini:
- trafik web customer tetap bisa masuk ke `80/443`
- hanya `Server 1` yang boleh mengakses Incus API dan Caddy Admin API

## 6.9 Catatan DNS

Supaya deployment berjalan sesuai arsitektur:

### Domain backend API
- `api.example.com` arahkan ke IP publik `Server 1`

### Domain customer
- domain atau wildcard domain customer arahkan ke IP publik `Server 2`

Contoh:
- `api.example.com` -> `203.0.113.10`
- `customer-domain.com` -> `203.0.113.20`
- `*.vps.example.com` -> `203.0.113.20`

## 7. Hubungkan Server 1 ke Server 2

Setelah dua server siap, verifikasi koneksi antar service:

### Dari Server 1 ke PostgreSQL lokal
- backend harus bisa connect ke `127.0.0.1:5432`

### Dari Server 1 ke Incus di Server 2
- backend harus bisa connect ke `https://203.0.113.20:8443`
- certificate harus valid

### Dari Server 1 ke Caddy Admin API di Server 2
- backend harus bisa connect ke `http://203.0.113.20:2019`

### Dari internet ke Server 2
- trafik `80/443` untuk domain customer harus menuju `Server 2`

Contoh uji konektivitas dasar dari `Server 1`:

```bash
curl -I http://203.0.113.20:2019
curl -vk https://203.0.113.20:8443
```

Kalau `8443` butuh TLS client cert untuk verifikasi penuh, wajar jika test `curl` biasa belum sukses penuh. Yang penting backend nanti bisa connect memakai sertifikat yang benar.

## 8. Reverse Proxy untuk Backend API

Bagian ini terpisah dari Caddy domain customer.

Kalau backend API ingin dipublikasikan dengan domain seperti `api.example.com`, kamu punya 2 opsi:

### Opsi A: expose langsung dari proses backend
- backend listen di `0.0.0.0:8080`
- domain diarahkan ke `Server 1`
- lebih cepat untuk testing

### Opsi B: tambahkan reverse proxy di Server 1
- pasang `Caddy` atau `Nginx` di `Server 1`
- proxy ke `127.0.0.1:8080`
- lebih rapi untuk production

Untuk MVP, opsi B biasanya lebih nyaman.

## 9. Integrasi Telegram dan Payment

Agar flow bisnis lengkap, siapkan juga:

### Telegram bot customer
- bot memanggil endpoint backend `/telegram/...`
- `TELEGRAM_BOT_SECRET` harus sama antara bot dan backend

### Telegram bot alert admin
- isi `ADMIN_ALERT_TELEGRAM_BOT_TOKEN`
- isi `ADMIN_ALERT_TELEGRAM_CHAT_ID`

### Pakasir
- isi `PAKASIR_PROJECT_SLUG`
- isi `PAKASIR_API_KEY`
- arahkan webhook payment ke endpoint backend:

```text
/payments/pakasir/webhook
```

Kalau backend API dipublikasikan di `api.example.com`, maka contoh webhook menjadi:

```text
https://api.example.com/payments/pakasir/webhook
```

## 10. Checklist Verifikasi Akhir

Sebelum dipakai, cek satu per satu:

- `Server 1` bisa menjalankan backend tanpa error config
- PostgreSQL bisa diakses dan semua migration sudah masuk
- admin pertama berhasil dibuat dengan `cmd/admin-seed`
- `GET /health` dan `GET /healthz` merespons normal
- `Server 1` bisa connect ke remote Incus
- `Server 1` bisa connect ke Caddy Admin API
- `Server 2` bisa membuat dan menghapus container secara normal
- bridge network Incus aktif
- port forward / NAT customer bisa dibuat
- domain customer bisa diarahkan ke Caddy di `Server 2`
- bot Telegram bisa mengakses endpoint backend
- webhook payment bisa masuk ke backend

Tambahan uji yang bagus dilakukan:
- login admin berhasil
- create package dari admin berhasil
- create order test berhasil
- provisioning container test berhasil
- setup domain test berhasil
- action `start`, `stop`, `reboot`, dan `reinstall` berhasil
- alert admin tidak error saat dipicu

## 11. Urutan Pengerjaan yang Disarankan

Supaya tidak bingung, kerjakan dengan urutan ini:

1. Siapkan `Server 2` lebih dulu sampai Incus sehat.
2. Aktifkan bridge network dan uji create container manual.
3. Install Caddy di `Server 2` dan pastikan Admin API hidup.
4. Siapkan `Server 1`, install PostgreSQL dan backend.
5. Isi `.env` backend.
6. Apply migration database.
7. Seed admin pertama.
8. Hubungkan backend ke Incus dan Caddy.
9. Jalankan backend dan cek `/health`.
10. Baru lanjut integrasi bot Telegram, payment, dan uji flow beli VPS.

## 12. Rekomendasi Operasional

Untuk setup MVP ini, rekomendasi saya:
- simpan backend dan database di `Server 1`
- simpan seluruh workload container di `Server 2`
- jangan expose `5432`, `8443`, dan `2019` ke publik tanpa pembatasan firewall
- gunakan backup rutin untuk PostgreSQL
- monitor resource `Server 2` lebih ketat karena itu node utama container customer

Kalau nanti user bertambah banyak, upgrade berikutnya yang paling masuk akal biasanya:
- pisahkan PostgreSQL ke server khusus
- tambah monitoring infra yang lebih formal
- pertimbangkan pemisahan edge proxy dan node Incus bila trafik domain makin besar

## 13. Ringkasan Install per Server

Supaya paling gampang diingat:

### Server 1
Install:
- Go
- Git
- PostgreSQL
- opsional Caddy untuk reverse proxy API

Jalankan:
- backend `vps-nat`
- database PostgreSQL

### Server 2
Install:
- Incus
- Caddy

Jalankan:
- container customer
- NAT / network forward
- reverse proxy domain customer

Kalau tujuanmu hanya ingin cepat online untuk MVP kecil, pembagian ini adalah titik awal yang paling aman dan paling gampang dirawat.
