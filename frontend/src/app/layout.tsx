import type { Metadata } from "next";
import { Geist, IBM_Plex_Mono, JetBrains_Mono, Lora } from "next/font/google";
import type { ReactNode } from "react";
import { ClerkProvider } from "@clerk/nextjs";
import { Providers } from "@/components/providers";
import "./globals.css";
import "./dashboard.css";
import "./garden.css";
import "./pipeline.css";
import "./deploy.css";
import "./apps.css";
import "./app-page.css";
import "./run.css";
import "./forms.css";
import "./type.css";
import "./seasons.css";
import "./palette.css";
import "./landing.css";

const heroFont = Lora({
  subsets: ["latin"],
  weight: ["400", "500"],
  style: ["normal", "italic"],
  display: "swap",
  variable: "--font-hero",
});

const monoFont = JetBrains_Mono({
  subsets: ["latin"],
  weight: ["400", "500"],
  display: "swap",
  variable: "--font-code",
});

const sansFont = Geist({
  subsets: ["latin"],
  display: "swap",
  variable: "--font-landing",
});

const labelFont = IBM_Plex_Mono({
  subsets: ["latin"],
  weight: ["400", "500"],
  display: "swap",
  variable: "--font-label",
});

export const metadata: Metadata = {
  title: "Sprout — small apps, live at a public URL",
  description: "Deploy small, purpose-built apps to a public URL without managing the cloud around them.",
};

export default function RootLayout({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <html lang="en" data-scroll-behavior="smooth" className={`${heroFont.variable} ${monoFont.variable} ${sansFont.variable} ${labelFont.variable}`}>
      <body>
        <ClerkProvider
          signInUrl="/sign-in"
          signUpUrl="/sign-up"
          signInForceRedirectUrl="/start"
          signUpForceRedirectUrl="/start"
        >
          <Providers>{children}</Providers>
        </ClerkProvider>
      </body>
    </html>
  );
}
