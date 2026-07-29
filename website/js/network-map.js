/**
 * Interactive world map for VeritasVPN network locations.
 * Equirectangular projection on a 1000×500 viewBox.
 */

/** @typedef {{ id: string, name: string, city: string, country: string, lat: number, lng: number, status: 'live' | 'soon' }} NetworkLocation */

/** @type {NetworkLocation[]} */
export const NETWORK_LOCATIONS = [
  {
    id: 'py-asu',
    name: 'Paraguay',
    city: 'Asunción metro',
    country: 'PY',
    lat: -25.3,
    lng: -57.6,
    status: 'live',
  },
];

/**
 * @param {number} lat
 * @param {number} lng
 * @param {number} [width=1000]
 * @param {number} [height=500]
 */
export function project(lat, lng, width = 1000, height = 500) {
  const x = ((lng + 180) / 360) * width;
  const y = ((90 - lat) / 180) * height;
  return { x, y };
}

/**
 * Minimal landmass silhouettes (decorative, not cadastral).
 * Enough to read as a world map behind location markers.
 */
const LAND_PATHS = [
  // North America
  'M148,118 L210,95 L268,108 L292,148 L275,195 L230,210 L185,188 L155,155 Z',
  // South America (includes Paraguay at ~340,320)
  'M255,240 L305,228 L345,255 L355,300 L340,360 L310,400 L275,375 L250,310 Z',
  // Europe
  'M480,105 L535,95 L560,125 L545,155 L500,160 L475,135 Z',
  // Africa
  'M495,175 L555,165 L580,220 L565,310 L520,330 L485,280 L490,210 Z',
  // Asia
  'M560,90 L720,85 L780,130 L760,200 L680,210 L600,175 L555,130 Z',
  // SE Asia / Australia
  'M730,250 L790,245 L820,290 L800,330 L750,325 L725,285 Z',
  'M780,360 L850,355 L875,395 L840,420 L790,405 Z',
];

/**
 * @param {HTMLElement} mount
 * @param {{ variant?: 'hero' | 'panel', locations?: NetworkLocation[] }} [opts]
 */
export function mountNetworkMap(mount, opts = {}) {
  const variant = opts.variant || 'panel';
  const locations = opts.locations || NETWORK_LOCATIONS;
  const live = locations.filter((l) => l.status === 'live');

  const markers = locations
    .map((loc) => {
      const { x, y } = project(loc.lat, loc.lng);
      const labelY = y - 18;
      if (loc.status !== 'live') {
        return `<g class="marker soon" transform="translate(${x.toFixed(1)},${y.toFixed(1)})" opacity="0.35">
          <circle r="4" fill="currentColor"/>
        </g>`;
      }
      return `<g class="marker live" data-location="${loc.id}">
        <circle class="marker-ring" cx="${x.toFixed(1)}" cy="${y.toFixed(1)}" r="6"/>
        <circle class="marker-ring" cx="${x.toFixed(1)}" cy="${y.toFixed(1)}" r="6" style="animation-delay:0.8s"/>
        <circle class="marker-core" cx="${x.toFixed(1)}" cy="${y.toFixed(1)}" r="5"/>
        <text class="marker-label" x="${(x + 12).toFixed(1)}" y="${labelY.toFixed(1)}">${loc.name}</text>
        <text class="marker-sub" x="${(x + 12).toFixed(1)}" y="${(labelY + 12).toFixed(1)}">${loc.city} · live</text>
      </g>`;
    })
    .join('');

  const land = LAND_PATHS.map((d) => `<path class="land" d="${d}"/>`).join('');

  const grid = [0.25, 0.5, 0.75]
    .map(
      (t) =>
        `<line class="grid-line" x1="0" y1="${500 * t}" x2="1000" y2="${500 * t}"/>
         <line class="grid-line" x1="${1000 * t}" y1="0" x2="${1000 * t}" y2="500"/>`
    )
    .join('');

  mount.innerHTML = `
    <svg class="network-map" viewBox="0 0 1000 500" role="img"
      aria-label="World map showing ${live.length} live VeritasVPN location${live.length === 1 ? '' : 's'}: ${live.map((l) => l.name).join(', ')}">
      <rect width="1000" height="500" fill="transparent"/>
      ${grid}
      <g class="lands">${land}</g>
      <g class="markers">${markers}</g>
    </svg>
  `;

  if (variant === 'hero') {
    mount.setAttribute('aria-hidden', 'true');
  }
}
