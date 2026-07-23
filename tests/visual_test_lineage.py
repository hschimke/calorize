import os
import sys
import time
import json
import threading
import http.server
import socketserver
import subprocess
import urllib.request
import urllib.error
from playwright.sync_api import sync_playwright

PORT = 8085
GO_PORT = 8086
BASE_URL = f"http://localhost:{PORT}"
GO_BASE_URL = f"http://localhost:{GO_PORT}"
DB_PATH = "./test_visual.db"
SNAPSHOT_DIR = os.path.join(os.path.dirname(__file__), "visual_snapshots")
STATIC_DIR = os.path.join(os.path.dirname(__file__), "..", "static-web")

def log(msg):
    print(f"[VISUAL TEST] {msg}")

class ProxyStaticServer(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=STATIC_DIR, **kwargs)

    def log_message(self, format, *args):
        pass # Suppress noisy HTTP server logs

    def do_GET(self):
        if self.path.startswith("/api/"):
            self.proxy()
        else:
            super().do_GET()

    def do_POST(self):
        if self.path.startswith("/api/"):
            self.proxy()
        else:
            self.send_error(405)

    def do_PUT(self):
        if self.path.startswith("/api/"):
            self.proxy()
        else:
            self.send_error(405)

    def do_DELETE(self):
        if self.path.startswith("/api/"):
            self.proxy()
        else:
            self.send_error(405)

    def proxy(self):
        content_len = int(self.headers.get('Content-Length', 0))
        body = self.rfile.read(content_len) if content_len > 0 else None

        target_url = f"{GO_BASE_URL}{self.path}"
        req = urllib.request.Request(target_url, data=body, method=self.command)
        for k, v in self.headers.items():
            if k.lower() not in ['host', 'content-length']:
                req.add_header(k, v)
        if body:
            req.add_header('Content-Length', str(len(body)))

        try:
            with urllib.request.urlopen(req) as resp:
                self.send_response(resp.status)
                for k, v in resp.headers.items():
                    if k.lower() not in ['transfer-encoding', 'content-length']:
                        self.send_header(k, v)
                resp_body = resp.read()
                self.send_header('Content-Length', str(len(resp_body)))
                self.end_headers()
                self.wfile.write(resp_body)
        except urllib.error.HTTPError as e:
            self.send_response(e.code)
            for k, v in e.headers.items():
                if k.lower() not in ['transfer-encoding', 'content-length']:
                    self.send_header(k, v)
            err_body = e.read()
            self.send_header('Content-Length', str(len(err_body)))
            self.end_headers()
            self.wfile.write(err_body)
        except Exception as ex:
            self.send_error(502, f"Proxy error: {ex}")

def start_proxy_server():
    socketserver.TCPServer.allow_reuse_address = True
    httpd = socketserver.TCPServer(("", PORT), ProxyStaticServer)
    thread = threading.Thread(target=httpd.serve_forever, daemon=True)
    thread.start()
    return httpd

def wait_for_server(url, timeout=15):
    start = time.time()
    while time.time() - start < timeout:
        try:
            with urllib.request.urlopen(f"{url}/api/v1/healthz") as response:
                if response.status == 200:
                    return True
        except Exception:
            pass
        time.sleep(0.3)
    return False

def http_post(path, data):
    req = urllib.request.Request(
        f"{BASE_URL}/api/v1{path}",
        data=json.dumps(data).encode('utf-8'),
        headers={'Content-Type': 'application/json'}
    )
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read().decode('utf-8'))

def http_put(path, data):
    req = urllib.request.Request(
        f"{BASE_URL}/api/v1{path}",
        data=json.dumps(data).encode('utf-8'),
        headers={'Content-Type': 'application/json'},
        method='PUT'
    )
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read().decode('utf-8'))

def seed_data():
    log("Seeding copy lineage test data...")
    root = http_post("/foods", {
        "name": "Almond Butter Granola Bar",
        "calories": 220,
        "protein": 6,
        "carbs": 28,
        "fat": 9,
        "type": "food",
        "measurement_unit": "bar",
        "measurement_amount": 1
    })
    root_id = root["id"]
    log(f"Created root food: {root_id}")

    copy1 = http_post(f"/foods/{root_id}/copy", {})
    copy1_id = copy1["id"]
    http_put(f"/foods/{copy1_id}", {
        "name": "Dark Chocolate Granola Bar",
        "calories": 230,
        "protein": 7,
        "carbs": 26,
        "fat": 10,
        "type": "food",
        "measurement_unit": "bar",
        "measurement_amount": 1
    })
    log(f"Created and edited copy1: {copy1_id}")

    copy2 = http_post(f"/foods/{copy1_id}/copy", {})
    copy2_id = copy2["id"]
    http_put(f"/foods/{copy2_id}", {
        "name": "Dark Chocolate Granola Bar (High Protein)",
        "calories": 250,
        "protein": 14,
        "carbs": 24,
        "fat": 10,
        "type": "food",
        "measurement_unit": "bar",
        "measurement_amount": 1
    })
    log(f"Created copy2 (High Protein): {copy2_id}")

    copy3 = http_post(f"/foods/{copy1_id}/copy", {})
    copy3_id = copy3["id"]
    http_put(f"/foods/{copy3_id}", {
        "name": "Dark Chocolate Granola Bar (Low Sugar)",
        "calories": 210,
        "protein": 7,
        "carbs": 18,
        "fat": 11,
        "type": "food",
        "measurement_unit": "bar",
        "measurement_amount": 1
    })
    log(f"Created copy3 (Low Sugar): {copy3_id}")

    yesterday = "2026-07-22"
    today = "2026-07-23"
    tomorrow = "2026-07-24"

    log_entry1 = http_post("/logs", {
        "food_id": copy1_id,
        "amount": 1,
        "meal_tag": "snack",
        "logged_at": f"{yesterday}T10:00:00Z"
    })
    log(f"Logged entry on {yesterday}: {log_entry1['id']}")

    http_post("/logs/copy", {
        "from_date": yesterday,
        "to_date": today,
        "meal_tags": ["snack"]
    })
    log(f"Copied logs from {yesterday} to {today}")

    http_post("/logs/copy", {
        "from_date": today,
        "to_date": tomorrow,
        "meal_tags": ["snack"]
    })
    log(f"Copied logs from {today} to {tomorrow}")

    return {
        "root_id": root_id,
        "copy1_id": copy1_id,
        "copy2_id": copy2_id,
        "copy3_id": copy3_id,
        "today": today,
        "tomorrow": tomorrow
    }

def main():
    os.makedirs(SNAPSHOT_DIR, exist_ok=True)
    if os.path.exists(DB_PATH):
        os.remove(DB_PATH)

    bin_path = os.path.join(os.path.dirname(__file__), "..", "bin", "api-server")
    log("Ensuring api-server binary is built...")
    subprocess.run(["go", "build", "-o", bin_path, "./cmd/api-server/main.go"], check=True)

    env = os.environ.copy()
    env["DEV_AUTH"] = "true"
    env["PORT"] = str(GO_PORT)
    env["DB_PATH"] = DB_PATH

    log(f"Starting Go API server process on port {GO_PORT}...")
    server_proc = subprocess.Popen(
        [bin_path],
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE
    )

    log(f"Starting static & proxy server on port {PORT}...")
    proxy_httpd = start_proxy_server()

    try:
        if not wait_for_server(BASE_URL):
            log("Server failed to start in time.")
            out, err = server_proc.communicate(timeout=2)
            print("STDOUT:", out.decode())
            print("STDERR:", err.decode())
            sys.exit(1)

        log("Server is running and healthy.")
        data_ids = seed_data()

        with sync_playwright() as p:
            log("Launching Playwright browser...")
            browser = p.chromium.launch(headless=True)
            context = browser.new_context(viewport={"width": 1280, "height": 800})
            
            # Pre-set logged in state for dev user
            context.add_init_script("localStorage.setItem('user_id', 'dev_user_id');")
            page = context.new_page()

            # --- Scenario 1: My Foods list & row actions ---
            log("Navigating to My Foods page (/food-ui.html)...")
            page.goto(f"{BASE_URL}/food-ui.html")
            page.wait_for_selector("#foods-list li", timeout=5000)
            page.wait_for_timeout(500)

            snap1_path = os.path.join(SNAPSHOT_DIR, "01_my_foods_actions.png")
            page.screenshot(path=snap1_path)
            log(f"Saved snapshot 1: {snap1_path}")

            # --- Scenario 2: Lineage Modal ---
            log("Opening Lineage modal for 'Dark Chocolate Granola Bar (High Protein)'...")
            high_protein_row = page.locator("#foods-list li", has_text="Dark Chocolate Granola Bar (High Protein)")
            high_protein_row.get_by_role("button", name="Lineage").click()

            page.wait_for_selector("dialog.app-modal", timeout=5000)
            page.wait_for_selector(".lineage-tree", timeout=5000)
            page.wait_for_timeout(500)

            snap2_path = os.path.join(SNAPSHOT_DIR, "02_lineage_modal_tree.png")
            page.screenshot(path=snap2_path)
            log(f"Saved snapshot 2: {snap2_path}")

            # Close modal
            page.locator("button.modal-close").click()
            page.wait_for_timeout(300)

            # --- Scenario 3: Food Search Copy Button ---
            log("Navigating to Food Log page (/foodlog.html)...")
            page.goto(f"{BASE_URL}/foodlog.html")
            page.wait_for_selector("#food-search-container input", timeout=5000)
            page.fill("#food-search-container input", "Granola")
            page.wait_for_selector(".food-search-item button", timeout=5000)
            page.wait_for_timeout(500)

            snap3_path = os.path.join(SNAPSHOT_DIR, "03_food_search_copy_button.png")
            page.screenshot(path=snap3_path)
            log(f"Saved snapshot 3: {snap3_path}")

            # Blur search input
            page.keyboard.press("Escape")
            page.wait_for_timeout(300)

            # --- Scenario 4: Food Log Copied Badge ---
            log("Viewing Food Log copied badge...")
            page.wait_for_selector("button.badge-copied", timeout=5000)
            page.wait_for_timeout(500)

            snap4_path = os.path.join(SNAPSHOT_DIR, "04_log_copied_badge.png")
            page.screenshot(path=snap4_path)
            log(f"Saved snapshot 4: {snap4_path}")

            # --- Scenario 5: Log Copy History Modal ---
            log("Opening Copy History modal from copied badge...")
            page.locator("button.badge-copied").first.click()
            page.wait_for_selector("dialog.app-modal", timeout=5000)
            page.wait_for_timeout(500)

            snap5_path = os.path.join(SNAPSHOT_DIR, "05_log_copy_history_modal.png")
            page.screenshot(path=snap5_path)
            log(f"Saved snapshot 5: {snap5_path}")

            browser.close()
            log("Visual validation screenshots captured successfully!")

    finally:
        log("Shutting down servers...")
        proxy_httpd.shutdown()
        server_proc.terminate()
        try:
            server_proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            server_proc.kill()
        if os.path.exists(DB_PATH):
            os.remove(DB_PATH)

if __name__ == "__main__":
    main()
