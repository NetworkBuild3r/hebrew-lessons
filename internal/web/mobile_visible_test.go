package web

import (
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestMobileShowsOneBrandAndToggle loads the shell in a real browser at 390
// and checks that the desktop rail copies are not visible. Exactly one brand
// and one theme toggle may paint; the same holds at 1440, where the header
// copies are the ones hidden.
func TestMobileShowsOneBrandAndToggle(t *testing.T) {
	chrome := "/usr/bin/google-chrome-stable"
	if _, err := os.Stat(chrome); err != nil {
		t.Skip("chrome not available")
	}
	playwright := os.Getenv("PLAYWRIGHT_CORE")
	if playwright == "" {
		playwright = "/tmp/video/node_modules/playwright-core/index.mjs"
	}
	if _, err := os.Stat(playwright); err != nil {
		t.Skip("playwright-core not available")
	}

	srv := httptest.NewServer(New(nil))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	script := filepath.Join(dir, "visible.mjs")
	body := fmt.Sprintf(`import { chromium } from %q;
const browser = await chromium.launch({
  executablePath: %q,
  args: ["--no-sandbox", "--disable-dev-shm-usage"],
});
const page = await browser.newPage();
const vis = (sel) => [...document.querySelectorAll(sel)].filter((el) => {
  const s = getComputedStyle(el);
  const r = el.getBoundingClientRect();
  return s.display !== "none" && s.visibility !== "hidden" && Number(s.opacity) > 0 && r.width > 0 && r.height > 0;
}).length;
const failures = [];
for (const width of [390, 1440]) {
  await page.setViewportSize({ width, height: width === 390 ? 844 : 900 });
  for (const path of ["/", "/?due=0", "/?lang=en"]) {
    await page.goto(%q + path, { waitUntil: "networkidle" });
    const brand = await page.evaluate(vis, ".brand");
    const theme = await page.evaluate(vis, ".theme-toggle");
    if (brand !== 1 || theme !== 1) {
      failures.push(width + " " + path + " brand=" + brand + " theme=" + theme);
    }
  }
}
await browser.close();
if (failures.length) {
  console.error(failures.join("\n"));
  process.exit(1);
}
console.log("ok");
`, playwright, chrome, srv.URL)
	if err := os.WriteFile(script, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", script).CombinedOutput()
	if err != nil {
		t.Fatalf("visible chrome: %v\n%s", err, out)
	}
}
