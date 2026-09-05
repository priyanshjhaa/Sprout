export function GrowthSystem() {
  return (
    <svg
      className="growth-system"
      viewBox="0 0 700 650"
      role="presentation"
      aria-hidden="true"
      preserveAspectRatio="xMidYMid meet"
    >
      <defs>
        <linearGradient id="stem-gradient" x1="0" y1="1" x2="0" y2="0">
          <stop offset="0" stopColor="#8d7658" />
          <stop offset="0.42" stopColor="#77804c" />
          <stop offset="1" stopColor="#56663a" />
        </linearGradient>
        <linearGradient id="root-gradient" x1="0.5" y1="0" x2="0.5" y2="1">
          <stop offset="0" stopColor="#8a785e" />
          <stop offset="1" stopColor="#b59b7a" stopOpacity="0.25" />
        </linearGradient>
        <radialGradient id="seed-fill">
          <stop offset="0" stopColor="#8c985e" />
          <stop offset="0.7" stopColor="#6f7d47" />
          <stop offset="1" stopColor="#56643a" />
        </radialGradient>
        <filter id="seed-shadow" x="-80%" y="-80%" width="260%" height="260%">
          <feDropShadow dx="0" dy="8" stdDeviation="10" floodColor="#68553d" floodOpacity="0.2" />
        </filter>
        <filter id="leaf-shadow" x="-80%" y="-80%" width="260%" height="260%">
          <feDropShadow dx="0" dy="5" stdDeviation="7" floodColor="#536039" floodOpacity="0.14" />
        </filter>
      </defs>

      <g className="growth-soil">
        <ellipse cx="350" cy="355" rx="196" ry="57" />
        <path d="M185 355 C255 329 445 329 515 355" />
      </g>

      <g className="growth-roots">
        <path className="growth-path root root-center" pathLength="1" d="M350 354 C349 405 359 462 350 542" />
        <path className="growth-path root root-left" pathLength="1" d="M350 374 C316 404 278 413 233 452 C203 478 176 500 133 508" />
        <path className="growth-path root root-right" pathLength="1" d="M350 381 C391 410 424 430 469 458 C505 481 541 490 578 515" />
        <path className="growth-path root root-fine-left" pathLength="1" d="M273 417 C250 448 249 486 221 520" />
        <path className="growth-path root root-fine-right" pathLength="1" d="M439 438 C461 472 461 508 492 538" />
        <path className="growth-path root root-fine-edge" pathLength="1" d="M213 467 C176 466 151 452 116 458" />
      </g>

      <g className="growth-plant">
        <path className="growth-path stem stem-main" pathLength="1" d="M350 350 C350 310 343 277 351 242 C356 218 370 195 371 164" />
        <path className="growth-path stem branch-left" pathLength="1" d="M349 286 C317 269 287 245 258 214" />
        <path className="growth-path stem branch-right" pathLength="1" d="M351 253 C388 240 421 218 447 188" />
        <path className="growth-path stem branch-wide-left" pathLength="1" d="M348 314 C303 311 264 315 222 300" />
        <path className="growth-path stem branch-wide-right" pathLength="1" d="M351 302 C405 301 450 314 494 301" />

        <g className="growth-leaf leaf-one" filter="url(#leaf-shadow)">
          <path d="M257 215 C223 204 218 171 225 156 C258 155 281 179 257 215Z" />
          <path d="M258 213 C249 192 240 178 227 162" />
        </g>
        <g className="growth-leaf leaf-two" filter="url(#leaf-shadow)">
          <path d="M446 188 C456 151 489 143 506 151 C505 184 480 207 446 188Z" />
          <path d="M449 187 C470 175 486 164 500 154" />
        </g>
        <g className="growth-leaf leaf-three" filter="url(#leaf-shadow)">
          <path d="M222 300 C190 296 179 272 184 255 C214 251 236 269 222 300Z" />
          <path d="M221 298 C207 280 197 269 187 258" />
        </g>
        <g className="growth-leaf leaf-four" filter="url(#leaf-shadow)">
          <path d="M494 301 C514 272 542 272 557 282 C548 312 522 324 494 301Z" />
          <path d="M497 300 C519 295 535 289 551 284" />
        </g>
      </g>

      <g className="seed-halo">
        <circle cx="350" cy="350" r="54" />
        <circle cx="350" cy="350" r="32" />
      </g>
      <g className="seed" filter="url(#seed-shadow)">
        <ellipse cx="350" cy="350" rx="18" ry="24" fill="url(#seed-fill)" transform="rotate(18 350 350)" />
        <path d="M343 367 C350 351 354 341 361 331" />
      </g>

      <g className="growth-network">
        <path className="growth-path network-line network-left" pathLength="1" d="M221 300 C164 306 121 283 77 246" />
        <path className="growth-path network-line network-right" pathLength="1" d="M494 301 C551 298 596 268 628 228" />
        <path className="growth-path network-line network-bottom" pathLength="1" d="M469 458 C523 450 581 426 631 382" />
        <circle className="network-node network-node-one" cx="76" cy="245" r="7" />
        <circle className="network-node network-node-two" cx="629" cy="227" r="7" />
        <circle className="network-node network-node-three" cx="632" cy="381" r="7" />
      </g>

      <g className="sap-particles">
        <circle className="sap sap-one" cx="350" cy="346" r="3" />
        <circle className="sap sap-two" cx="350" cy="302" r="2.5" />
        <circle className="sap sap-three" cx="351" cy="258" r="2" />
      </g>
    </svg>
  );
}

