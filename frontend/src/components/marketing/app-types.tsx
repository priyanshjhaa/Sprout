"use client";

import { useLoopStep } from "@/components/marketing/use-loop-step";

const COLUMNS = 16;

// One hidden word per row, at a fixed column, the rest filled with letters.
const words = [
  { word: "STATICSITE", label: "Static sites", column: 3 },
  { word: "NODEAPI", label: "Node.js APIs", column: 8 },
  { word: "DASHBOARD", label: "Internal dashboards", column: 1 },
  { word: "WEBHOOK", label: "Webhook receivers", column: 6 },
  { word: "CHATBOT", label: "Chat bots", column: 2 },
  { word: "PORTFOLIO", label: "Portfolios", column: 7 },
  { word: "INTERNALTOOL", label: "Internal tools", column: 0 },
  { word: "PROTOTYPE", label: "Prototypes", column: 5 },
];

const ALPHABET = "ABCDEFGHIJKLMNOPQRSTUVWXYZ";

function noise(seed: number) {
  const value = Math.sin(seed * 78.233) * 43758.5453;
  return value - Math.floor(value);
}

const rows = words.map(({ word, column }, row) =>
  Array.from({ length: COLUMNS }, (_, index) => {
    const inWord = index >= column && index < column + word.length;
    return {
      letter: inWord ? word[index - column] : ALPHABET[Math.floor(noise(row * COLUMNS + index + 1) * 26)],
      inWord,
    };
  }),
);

export function AppTypes() {
  const [ref, step] = useLoopStep<HTMLDivElement>(words.length, 1500, 0);

  return (
    <section className="lp-types" aria-labelledby="types-title">
      <div className="lp-container lp-types-inner">
        <header data-reveal>
          <h2 id="types-title" className="lp-heading">
            Run any small app
          </h2>
          <p className="lp-lede">
            If it is a static site or one web process that answers on /health, Sprout can run it.
          </p>
          <p className="lp-types-current" aria-hidden="true">
            <span className="lp-mono">Now growing</span>
            <b key={step}>{words[step].label}</b>
          </p>
          <ul className="lp-types-list">
            {words.map((entry) => (
              <li key={entry.word}>{entry.label}</li>
            ))}
          </ul>
        </header>

        <div className="lp-letters lp-card" ref={ref} aria-hidden="true" data-reveal>
          {rows.map((cells, row) => (
            <div key={row} className="lp-letters-row" data-active={row === step || undefined}>
              {cells.map((cell, index) => (
                <span
                  key={index}
                  data-word={cell.inWord || undefined}
                  style={{ "--c": index - words[row].column } as React.CSSProperties}
                >
                  {cell.letter}
                </span>
              ))}
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
