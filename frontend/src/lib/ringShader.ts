// The ring's light. Paper's liquid-metal shader sweeps straight stripes across the whole card, so on a
// thin rim the light seems to come and go from different directions. This one measures how far along
// the card's outline each pixel is and lays the light along that distance: it travels around the rim
// in one direction, and two fronts on opposite sides keep the whole outline lit.
export const ringShader = `#version 300 es
// mediump, as in Paper's own shaders: ShaderMount's vertex shader declares u_resolution mediump, and a
// real GPU refuses to link the program when the two stages disagree on a uniform's precision (a software
// renderer does not, which is how this shipped broken).
precision mediump float;

uniform vec2 u_resolution;
uniform float u_pixelRatio;
uniform float u_time;
uniform vec4 u_colorBack;
uniform vec4 u_colorTint;
uniform float u_radius; // the card's corner radius, in CSS pixels

out vec4 fragColor;

const float TAU = 6.28318530718;
const float FRONTS = 2.0;   // lit places around the outline
const float LAP_RATE = 0.25; // fronts per second at speed 1

// Distance along the outline of a rounded rectangle, from the top centre clockwise, as a share of its
// length. p is relative to the centre with y up.
float along(vec2 p, vec2 half_, float r) {
  vec2 m = half_ - r;
  vec2 a = abs(p);
  vec2 d = a - m;
  float quarterArc = r * 1.5707963;
  float u;
  if (d.x <= 0.0) u = a.x;                                     // top or bottom edge
  else if (d.y <= 0.0) u = m.x + quarterArc + (m.y - a.y);     // left or right edge
  else u = m.x + r * atan(d.x, d.y);                           // corner
  float quarter = m.x + quarterArc + m.y;
  float s = p.y >= 0.0 ? (p.x >= 0.0 ? u : 4.0 * quarter - u) : (p.x >= 0.0 ? 2.0 * quarter - u : 2.0 * quarter + u);
  return s / (4.0 * quarter);
}

// The metal at one point of the cycle. The body colour shows all the way round, only a little darker
// in the troughs, so the ring always reads as that metal; the highlight colour rides on the peaks and a
// trace of white sits on the very top. A trough as dark as the page would leave a thin lit streak and
// no colour.
vec3 metal(float h, vec3 body, vec3 tint) {
  vec3 c = body * mix(0.62, 1.15, smoothstep(0.0, 0.65, h));
  c = mix(c, tint, 0.85 * smoothstep(0.6, 0.97, h));
  return mix(c, vec3(1.0), 0.15 * smoothstep(0.96, 1.0, h));
}

void main() {
  vec2 p = gl_FragCoord.xy - 0.5 * u_resolution;
  float s = along(p, 0.5 * u_resolution, u_radius * u_pixelRatio);

  // A slow wobble in the phase makes the bands breathe like liquid instead of marching evenly.
  float phase = FRONTS * s - LAP_RATE * u_time + 0.03 * sin(TAU * 5.0 * s + 1.7 * u_time);
  // The three colour channels are a hair apart, which gives the highlights their coloured fringes.
  vec3 h = 0.5 + 0.5 * cos(TAU * (phase + vec3(0.012, 0.0, -0.012)));
  vec3 body = u_colorBack.rgb;
  vec3 tint = u_colorTint.rgb;
  fragColor = vec4(metal(h.r, body, tint).r, metal(h.g, body, tint).g, metal(h.b, body, tint).b, 1.0);
}
`;
