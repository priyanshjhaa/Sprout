import { expect, test } from "@playwright/test";

test.describe("sign-in and sign-up", () => {
  test("the switch moves between modes and keeps an invitation", async ({ page }) => {
    await page.goto("/sign-in?invitation=1");
    const modes = page.getByRole("navigation", { name: "Authentication mode" });
    await expect(modes.getByRole("link", { name: "Sign In" })).toHaveAttribute("aria-current", "page");
    await expect(modes.getByRole("link", { name: "Sign Up" })).toHaveAttribute("href", "/sign-up?invitation=1");

    await modes.getByRole("link", { name: "Sign Up" }).click();
    await expect(page).toHaveURL(/\/sign-up\?invitation=1$/);
    await expect(modes.getByRole("link", { name: "Sign Up" })).toHaveAttribute("aria-current", "page");
  });

  test("social buttons stack as full-width pills", async ({ page }) => {
    await page.goto("/sign-in");
    const pills = page.locator(".auth .auth-clerk .cl-socialButtonsBlockButton");
    await expect(pills.first()).toBeVisible();
    const boxes = await pills.evaluateAll((elements) => elements.map((element) => element.getBoundingClientRect().toJSON()));
    expect(boxes.length).toBeGreaterThanOrEqual(2);
    // Stacked: same left edge and width, one below the other.
    expect(Math.abs(boxes[0].left - boxes[1].left)).toBeLessThan(1);
    expect(Math.abs(boxes[0].width - boxes[1].width)).toBeLessThan(1);
    expect(boxes[1].top).toBeGreaterThan(boxes[0].bottom - 1);
  });

  test("Clerk's last-used badge stays inside its pill", async ({ page }) => {
    await page.goto("/sign-in");
    const pill = page.locator(".auth .auth-clerk .cl-socialButtonsBlockButton").first();
    await expect(pill).toBeVisible();
    // A fresh browser has no last-used method, so add the badge Clerk would render, with
    // Clerk's default placement half outside the corner at a low specificity our fix overrides.
    await page.addStyleTag({ content: ".cl-lastAuthenticationStrategyBadge { position: absolute; top: -10px; right: -12px; }" });
    const fits = await pill.evaluate((button) => {
      const badge = document.createElement("span");
      badge.className = "cl-lastAuthenticationStrategyBadge";
      badge.textContent = "Last used";
      button.appendChild(badge);
      const inner = badge.getBoundingClientRect();
      const outer = button.getBoundingClientRect();
      return inner.top >= outer.top && inner.bottom <= outer.bottom && inner.left >= outer.left && inner.right <= outer.right;
    });
    expect(fits).toBe(true);
  });

  test("does not scroll sideways", async ({ page }) => {
    await page.goto("/sign-in");
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    expect(overflow).toBeLessThanOrEqual(0);
  });
});

test("the workspace sends signed-out visitors to sign in", async ({ page }) => {
  await page.goto("/workspace/acme");
  await expect(page).toHaveURL(/\/sign-in/);
});
