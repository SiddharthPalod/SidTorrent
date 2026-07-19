import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.jsx'

// --- Lazy glow that lerps toward the cursor each frame ---
// SPEED controls how quickly the glow catches up (0.03 = very lazy, 0.15 = snappy)
const SPEED = 0.06;

// Current glow position (starts at the CSS fallback: 70vw, 10vh)
let glowX = window.innerWidth  * 0.70;
let glowY = window.innerHeight * 0.10;

// Target position — updated instantly on every mouse move
let targetX = glowX;
let targetY = glowY;

document.addEventListener('mousemove', (e) => {
  targetX = e.clientX;
  targetY = e.clientY;
});

function animateGlow() {
  // Lerp: move a fraction of the remaining distance each frame
  glowX += (targetX - glowX) * SPEED;
  glowY += (targetY - glowY) * SPEED;

  document.body.style.setProperty('--mouse-x', `${glowX}px`);
  document.body.style.setProperty('--mouse-y', `${glowY}px`);

  requestAnimationFrame(animateGlow);
}

requestAnimationFrame(animateGlow);

createRoot(document.getElementById('root')).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
