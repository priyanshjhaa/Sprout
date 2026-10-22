// Clerk owns authentication; this only restyles its components to match the auth shell.
// Class names map Clerk's elements onto styles in auth.css.
export const authAppearance = {
  options: {
    elevation: "flush",
    socialButtonsVariant: "blockButton",
    socialButtonsPlacement: "top",
  },
  variables: {
    colorPrimary: "#2f5a2b",
    colorForeground: "#171717",
    colorMutedForeground: "rgb(38 35 35 / 0.6)",
    colorBackground: "#f5f5f2",
    colorInput: "#ffffff",
    fontFamily: "inherit",
    borderRadius: "12px",
  },
  elements: {
    rootBox: "auth-clerk",
    cardBox: "auth-clerk-box",
    card: "auth-clerk-card",
    socialButtons: "auth-socials",
    socialButtonsBlockButton: "auth-pill",
    dividerRow: "auth-divider",
    formFieldLabel: "auth-label",
    formFieldInput: "auth-input",
    formButtonPrimary: "auth-pill auth-pill--primary",
    footer: "auth-clerk-footer",
  },
} as const;
