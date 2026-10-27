import { expect, test } from "@playwright/test";

test.describe("landing page", () => {
  test("tells the story and offers one way in", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { level: 1, name: /safe place to live/i })).toBeVisible();

    // The nav keeps a single entry action; sign-in lives in the footer.
    const nav = page.locator(".lp-nav");
    await expect(nav.getByRole("link", { name: "Deploy an app" })).toHaveAttribute("href", "/start");
    await expect(nav.getByRole("link", { name: "Sign in" })).toHaveCount(0);

    for (const id of ["how", "deploy", "operate", "share", "ways-in"]) {
      await expect(page.locator(`#${id}`)).toHaveCount(1);
    }
    await expect(page.locator("footer").getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/sign-in");
  });

  test("marks only the dashboard and the CLI as available", async ({ page }) => {
    await page.goto("/#ways-in");
    const statuses = page.locator(".lp-book-status");
    await expect(statuses).toHaveCount(4);
    const available = page.locator("li", { has: page.locator(".lp-book-status[data-available]") });
    await expect(available.getByRole("heading", { level: 3 })).toHaveText(["The Dashboard", "The CLI"]);
  });

  test("does not scroll sideways", async ({ page }) => {
    await page.goto("/");
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    expect(overflow).toBeLessThanOrEqual(0);
  });

  test("keeps every section readable with reduced motion", async ({ browser }) => {
    const context = await browser.newContext({ reducedMotion: "reduce" });
    const page = await context.newPage();
    await page.goto("/");
    // Revealed content is never hidden when motion is reduced, even before it scrolls into view.
    const hidden = await page.evaluate(() =>
      [...document.querySelectorAll("[data-reveal]")].filter((element) => getComputedStyle(element).opacity === "0").length,
    );
    expect(hidden).toBe(0);
    await context.close();
  });
});
