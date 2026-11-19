// Drives the landing page dial: it seeks to a preview station, holds there, then seeks to the next one.
import { Tween, prefersReducedMotion } from 'svelte/motion'
import { cubicInOut } from 'svelte/easing'

// How long the needle takes to travel between two stations
const TUNE_DURATION_MS = 2400
// How long the dial rests on a station before moving on
const HOLD_DURATION_MS = 4200
// How close (in station numbers) the needle must be to count as locked on
const LOCK_TOLERANCE = 0.5
// Stations are numbered 000 to 999
const DIAL_MAX = 999
// A numbered label sits on the band every this many stations
const DIAL_LABEL_STEP = 25

/** Width of one station number on the band, in pixels. The tick spacing in Tuner.svelte is built from it. */
export const DIAL_UNIT_PX = 4
/** How many digits the station readout shows. */
export const READOUT_DIGITS = 3
/** Every station number that gets a label on the band. */
export const DIAL_LABELS = Array.from({ length: Math.floor(DIAL_MAX / DIAL_LABEL_STEP) + 1 }, (_, i) => i * DIAL_LABEL_STEP)
/** Total width of the band, in pixels. */
export const DIAL_WIDTH_PX = DIAL_MAX * DIAL_UNIT_PX

/** Placeholder stations the dial cycles through until real ones exist. */
export const PREVIEW_STATIONS = [
  { number: 88, name: 'LATE NIGHT' },
  { number: 247, name: 'HEAVY ROTATION' },
  { number: 104, name: 'DEEP CUTS' },
  { number: 512, name: 'SLOW SUNDAY' },
  { number: 303, name: 'WAREHOUSE' },
]

/** Reactive dial state: which preview station is selected and where the needle currently sits. */
export class Tuner {
  #index = $state(0)
  #needle = new Tween(PREVIEW_STATIONS[0].number, { duration: TUNE_DURATION_MS, easing: cubicInOut })

  /** The preview station the dial is on, or travelling towards. */
  get station() {
    return PREVIEW_STATIONS[this.#index]
  }

  /** Needle position as a fractional station number. */
  get position() {
    return this.#needle.current
  }

  /** True once the needle has settled on the selected station. */
  get locked() {
    return Math.abs(this.#needle.current - this.station.number) < LOCK_TOLERANCE
  }

  /**
   * Starts cycling through the preview stations. Does nothing for visitors who asked for reduced motion.
   * @returns {(() => void) | undefined} stops the cycle
   */
  start() {
    if (prefersReducedMotion.current) return

    const timer = setInterval(() => {
      this.#index = (this.#index + 1) % PREVIEW_STATIONS.length
      this.#needle.target = this.station.number
    }, TUNE_DURATION_MS + HOLD_DURATION_MS)
    return () => clearInterval(timer)
  }
}
