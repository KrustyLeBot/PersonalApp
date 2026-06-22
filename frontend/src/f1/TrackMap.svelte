<script>
  import { onMount, onDestroy } from 'svelte';
  import L from 'leaflet';
  import 'leaflet/dist/leaflet.css';

  // Telemetry-space map: the track outline and the cars come from the SAME
  // OpenF1 /location coordinate system, so cars sit exactly on the track. We use
  // Leaflet's CRS.Simple (flat x/y plane, no geography) and just plot points.
  //
  // outline: [{x,y}...] one lap of telemetry tracing the circuit
  // cars: [{ number, x, y, color, acronym, position }]
  export let outline = [];
  export let cars = [];

  let mapEl;
  let map = null;
  let trackLine = null;
  let markers = {};      // driver_number -> L.Marker
  let bounds = null;     // telemetry bounds for scaling
  let ready = false;

  // OpenF1 telemetry is roughly in decimetres. We scale into Leaflet "pixels"
  // and flip Y (telemetry Y grows downward; Leaflet lat grows upward).
  const SCALE = 0.01;
  function toLatLng(x, y) {
    return [-y * SCALE, x * SCALE];
  }

  onMount(() => {
    map = L.map(mapEl, {
      crs: L.CRS.Simple,
      zoomControl: false,
      attributionControl: false,
      zoomSnap: 0.25,
      minZoom: -5,
      maxZoom: 5,
    });
    mapEl.style.background = '#0a0e17';
    drawOutline();
    ready = true;
    renderCars();
  });

  onDestroy(() => { if (map) map.remove(); });

  function drawOutline() {
    if (!map) return;
    if (trackLine) { map.removeLayer(trackLine); trackLine = null; }
    if (!outline || outline.length < 3) return;
    const latlngs = outline.map(p => toLatLng(p.x, p.y));
    trackLine = L.polyline(latlngs, {
      color: '#475569', weight: 8, opacity: 0.9,
      lineJoin: 'round', lineCap: 'round',
    }).addTo(map);
    // A thin bright centre line on top for a track-like look.
    L.polyline(latlngs, { color: '#94a3b8', weight: 2, opacity: 0.5 }).addTo(map);
    bounds = trackLine.getBounds();
    map.fitBounds(bounds, { padding: [24, 24] });
  }

  function renderCars() {
    if (!map) return;
    const seen = new Set();
    let pts = [];

    for (const car of cars) {
      if (car.x == null || car.y == null) continue;
      const ll = toLatLng(car.x, car.y);
      pts.push(ll);
      seen.add(car.number);

      const icon = L.divIcon({
        className: 'car-dot-wrap',
        html: `<div class="car-dot" style="background:${car.color}">${car.acronym ?? car.number}</div>`,
        iconSize: [30, 16],
        iconAnchor: [15, 8],
      });
      if (markers[car.number]) {
        markers[car.number].setLatLng(ll).setIcon(icon);
      } else {
        markers[car.number] = L.marker(ll, { icon, keyboard: false }).addTo(map);
      }
    }

    for (const num of Object.keys(markers)) {
      if (!seen.has(Number(num))) {
        map.removeLayer(markers[num]);
        delete markers[num];
      }
    }

    // No outline available → fit to the cars so they're visible.
    if (!bounds && pts.length > 0) {
      map.fitBounds(L.latLngBounds(pts), { padding: [40, 40] });
    }
  }

  $: if (ready && outline) drawOutline();
  $: if (ready && cars) renderCars();
</script>

<div class="track-map" bind:this={mapEl}></div>

<style>
  .track-map {
    width: 100%;
    height: 360px;
    border-radius: 12px;
    border: 1px solid #334155;
    overflow: hidden;
    z-index: 0;
  }
  :global(.car-dot) {
    display: flex; align-items: center; justify-content: center;
    width: 30px; height: 16px;
    font-size: .6rem; font-weight: 800; color: #0a0e17;
    border-radius: 4px;
    border: 1px solid rgba(0,0,0,.4);
    box-shadow: 0 1px 3px rgba(0,0,0,.6);
    font-family: system-ui, sans-serif;
  }
  :global(.leaflet-container) { background: #0a0e17; }
</style>
