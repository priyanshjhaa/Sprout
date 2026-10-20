import type { AppStatus } from "@/types/domain";

// One plant per app. Its stage tells the lifecycle story at a glance:
// a seed waits, roots grow while it builds, a live app is in leaf, a failed one
// wilts, a paused one sleeps, an archived one is pressed flat.
const leaf = "M0 0 C8 -9 21 -8 26 1 C18 10 6 9 0 0Z";

function Leaf({ x, y, turn, fill }: { x: number; y: number; turn: number; fill: string }) {
  return <path d={leaf} fill={fill} transform={`translate(${x} ${y}) rotate(${turn})`} />;
}

export function Plant({ status }: { status: AppStatus }) {
  const green = "#7d8c4f";
  const light = "#9aa874";
  return (
    <svg className={`sp-plant sp-plant-${status}`} viewBox="0 0 80 120" width="80" height="120" aria-hidden="true" focusable="false">
      {status === "undeployed" && (
        <g>
          <ellipse cx="40" cy="98" rx="9" ry="5.5" fill="#7a5c3a" />
          <path d="M34 96 C37 93 43 93 46 96" stroke="#a98a5a" strokeWidth="1.6" fill="none" strokeLinecap="round" />
        </g>
      )}
      {status === "building" && (
        <g>
          <g className="sp-roots" stroke={light} strokeWidth="1.5" fill="none" strokeLinecap="round" strokeDasharray="3 3">
            <path d="M40 100 C40 108 36 112 30 118" /><path d="M40 100 C40 108 44 112 52 118" /><path d="M40 100 L40 118" />
          </g>
          <path d="M40 100 C40 94 40 90 40 84" stroke={green} strokeWidth="2.6" fill="none" strokeLinecap="round" />
          <Leaf x={40} y={86} turn={-30} fill={light} /><Leaf x={40} y={88} turn={-150} fill={green} />
        </g>
      )}
      {status === "running" && (
        <g className="sp-sway">
          <path d="M40 100 C40 80 38 62 40 34" stroke={green} strokeWidth="3" fill="none" strokeLinecap="round" />
          <Leaf x={40} y={84} turn={-25} fill={green} /><Leaf x={40} y={80} turn={-155} fill={light} />
          <Leaf x={40} y={64} turn={-30} fill={light} /><Leaf x={40} y={60} turn={-150} fill={green} />
          <Leaf x={40} y={46} turn={-35} fill={green} /><Leaf x={40} y={42} turn={-145} fill={light} />
          <circle cx="40" cy="31" r="5.5" fill="#d9a05b" /><circle cx="40" cy="31" r="2.4" fill="#f3d9a8" />
        </g>
      )}
      {status === "failed" && (
        <g>
          <path d="M40 100 C40 82 46 68 60 64" stroke="#8c7a4a" strokeWidth="3" fill="none" strokeLinecap="round" />
          <Leaf x={42} y={84} turn={40} fill="#a98a5a" /><Leaf x={46} y={74} turn={80} fill="#b87b4a" />
          <Leaf x={60} y={64} turn={110} fill="#a96a50" />
        </g>
      )}
      {status === "paused" && (
        <g opacity="0.75">
          <path d="M40 100 C40 90 39 80 40 66" stroke="#8d9473" strokeWidth="2.6" fill="none" strokeLinecap="round" />
          <Leaf x={40} y={86} turn={-20} fill="#a7ae8d" /><Leaf x={40} y={82} turn={-160} fill="#a7ae8d" />
          <Leaf x={40} y={70} turn={-30} fill="#b4baa0" /><Leaf x={40} y={67} turn={-150} fill="#b4baa0" />
          <text x="54" y="52" fontSize="11" fill="#8d9473" fontWeight="700">z</text><text x="62" y="42" fontSize="8" fill="#8d9473" fontWeight="700">z</text>
        </g>
      )}
      {status === "archived" && (
        <g opacity="0.7">
          <path d={leaf} transform="translate(22 92) rotate(-18)" fill="#d9ccae" stroke="#a98a5a" strokeWidth="1.2" />
          <path d="M22 92 L46 86" stroke="#a98a5a" strokeWidth="1" />
        </g>
      )}
    </svg>
  );
}
