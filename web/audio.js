// Web Audio API synthesizer — frequencies derived from standard tuning (A4 = 440 Hz)
// f(n) = 440 * 2^((n - 69) / 12) where n is the MIDI note number

const NOTE_FREQ = {
  'C2': 65.41,  'D2': 73.42,  'E2': 82.41,  'F2': 87.31,  'G2': 98.00,  'A2': 110.00, 'B2': 123.47,
  'C3': 130.81, 'D3': 146.83, 'E3': 164.81, 'F3': 174.61, 'G3': 196.00, 'A3': 220.00, 'B3': 246.94,
  'C4': 261.63, 'D4': 293.66, 'E4': 329.63, 'F4': 349.23, 'G4': 392.00, 'A4': 440.00, 'B4': 493.88,
  'C5': 523.25, 'D5': 587.33, 'E5': 659.25, 'F5': 698.46, 'G5': 783.99, 'A5': 880.00, 'B5': 987.77,
  'C6': 1046.50,'D6': 1174.66,'E6': 1318.51,'F6': 1396.91,'G6': 1567.98,'A6': 1760.00,'B6': 1975.53,
}

let audioCtx = null

function getAudioContext() {
  if (!audioCtx) {
    audioCtx = new (window.AudioContext || window.webkitAudioContext)()
  }
  if (audioCtx.state === 'suspended') audioCtx.resume()
  return audioCtx
}

function playSynth(volume = 1.0, velocity = 1.0, waveform = 'sine', frequency = 440) {
  const ctx = getAudioContext()
  const now = ctx.currentTime
  const gain = Math.min(velocity * volume * 0.5, 1.0)

  const osc = ctx.createOscillator()
  const gainNode = ctx.createGain()

  osc.type = waveform
  osc.frequency.value = frequency

  gainNode.gain.setValueAtTime(0, now)
  gainNode.gain.linearRampToValueAtTime(gain, now + 0.008)
  gainNode.gain.exponentialRampToValueAtTime(0.001, now + 0.5)

  osc.connect(gainNode)
  gainNode.connect(ctx.destination)

  osc.start(now)
  osc.stop(now + 0.55)
}

function playInstrument(instrument, volume, velocity, waveform, frequency) {
  const freq = frequency || 440
  playSynth(volume || 0.6, velocity || 0.8, waveform || 'sine', freq)
}
