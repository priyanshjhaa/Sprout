// The painted sky, soil, swaying grass, and drifting pollen behind the hero and the auth banner.
const pollen = Array.from({ length: 16 }, (_, index) => ({
  left: `${(index * 61) % 100}%`,
  delay: `${(index * 1.7) % 9}s`,
  duration: `${9 + (index % 5) * 2}s`,
  size: `${3 + (index % 3)}px`,
}));

export function LandscapeScene() {
  return (
    <div className="lp-hero-scene" aria-hidden="true">
      <div className="lp-hero-sky" />
      <div className="lp-hero-ground" />
      <div className="lp-hero-grass lp-hero-grass--left" />
      <div className="lp-hero-grass lp-hero-grass--right" />
      <div className="lp-hero-pollen">
        {pollen.map((particle, index) => (
          <i
            key={index}
            style={{
              left: particle.left,
              width: particle.size,
              height: particle.size,
              animationDelay: particle.delay,
              animationDuration: particle.duration,
            }}
          />
        ))}
      </div>
    </div>
  );
}
