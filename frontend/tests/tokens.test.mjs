import test from "node:test";
import assert from "node:assert/strict";
import { describeExpiry, describeLastUsed, displayHint, expiryChoices } from "../src/lib/tokens.ts";

const now = Date.parse("2026-10-14T12:00:00Z");
const ago = (ms) => new Date(now - ms).toISOString();
const ahead = (ms) => new Date(now + ms).toISOString();
const DAY = 24 * 60 * 60 * 1000;

test("expiry reads relatively and says so plainly when it has passed", () => {
  assert.equal(describeExpiry(ago(1), now), "Expired");
  assert.equal(describeExpiry(ahead(0), now), "Expired");
  assert.equal(describeExpiry(ahead(3 * 60 * 60 * 1000), now), "Expires within a day");
  assert.equal(describeExpiry(ahead(DAY + 1000), now), "Expires in 1 day");
  assert.equal(describeExpiry(ahead(89 * DAY + 5000), now), "Expires in 89 days");
  assert.equal(describeExpiry("not a date", now), "Expiry unknown");
});

test("last used reads relatively and never claims a use that did not happen", () => {
  assert.equal(describeLastUsed(null, now), "Never used");
  assert.equal(describeLastUsed(ago(5_000), now), "Used just now");
  assert.equal(describeLastUsed(ago(5 * 60_000), now), "Used 5 min ago");
  assert.equal(describeLastUsed(ago(3 * 60 * 60_000), now), "Used 3 hr ago");
  assert.equal(describeLastUsed(ago(DAY + 1000), now), "Used yesterday");
  assert.equal(describeLastUsed(ago(9 * DAY), now), "Used 9 days ago");
  assert.equal(describeLastUsed("garbage", now), "Used");
});

test("only the hint of a token is ever displayed, and the choices match the API's limits", () => {
  assert.equal(displayHint("wxyz"), "sprout_pat_…wxyz");
  assert.ok(expiryChoices.every((choice) => choice.days >= 1 && choice.days <= 365));
  assert.ok(expiryChoices.some((choice) => choice.days === 90), "the default must be offered");
});
