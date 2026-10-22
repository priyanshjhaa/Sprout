// Clerk owns authentication; this only restyles its components to match the auth shell.
export const authAppearance = {
  options: { elevation: "flush" },
  variables: {
    colorPrimary: "#687642",
    colorForeground: "#302b25",
    colorMutedForeground: "#70675d",
    colorBackground: "#fffaf2",
    colorInput: "#fffaf2",
    fontFamily: "inherit",
    borderRadius: "10px",
  },
  elements: {
    rootBox: { width: "100%" },
    cardBox: { width: "100%" },
    card: { width: "100%" },
  },
} as const;
