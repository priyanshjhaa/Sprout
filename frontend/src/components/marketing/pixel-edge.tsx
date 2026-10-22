// A stepped, pixelated edge where one surface dissolves into the next.
// Deterministic so server and client render the same squares.
const SIZE = 12;
const COLUMNS = 160;
const ROWS = 6;

function noise(seed: number) {
  const value = Math.sin(seed * 12.9898) * 43758.5453;
  return value - Math.floor(value);
}

const squares = Array.from({ length: COLUMNS * ROWS }, (_, index) => {
  const column = index % COLUMNS;
  const row = Math.floor(index / COLUMNS);
  // Lower rows are denser, so the edge reads as solid at the bottom.
  const density = ((row + 1) / ROWS) ** 1.6;
  return noise(index + 1) < density ? { x: column * SIZE, y: row * SIZE, delay: noise(index + 7) } : null;
}).filter((square) => square !== null);

export function PixelEdge({ className = "" }: { className?: string }) {
  return (
    <svg
      className={`lp-pixel-edge ${className}`}
      viewBox={`0 0 ${COLUMNS * SIZE} ${ROWS * SIZE}`}
      preserveAspectRatio="xMidYMax slice"
      aria-hidden="true"
    >
      {squares.map((square) => (
        <rect
          key={`${square.x}-${square.y}`}
          x={square.x}
          y={square.y}
          width={SIZE + 0.5}
          height={SIZE + 0.5}
          style={{ animationDelay: `${(square.delay * 900).toFixed(0)}ms` }}
        />
      ))}
    </svg>
  );
}
