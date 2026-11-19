<script>
  import { Tuner, DIAL_LABELS, DIAL_UNIT_PX, DIAL_WIDTH_PX, READOUT_DIGITS } from "./tuner.svelte.js";

  const DIAL_DESCRIPTION = "Animated radio dial tuning between preview stations";
  const LOCKED_LABEL = "ON AIR 24/7";
  const TUNING_LABEL = "TUNING";
  // One entry per equalizer bar; the value staggers each bar's animation
  const EQ_BARS = [0, 1, 2, 3, 4];

  const tuner = new Tuner();
  $effect(() => tuner.start());
</script>

<!-- The whole dial is decoration: assistive tech gets one description instead of a readout that never stops changing -->
<section role="img" aria-label={DIAL_DESCRIPTION} class="@container w-full flex flex-col gap-3 short:gap-1.5">
  <!-- READOUT ROW -->
  <div class="flex items-end justify-between gap-4">
    <div class="flex flex-col">
      <span class="text-[9px] sm:text-[10px] 2xl:text-sm tracking-[0.3em] text-muted">STATION</span>
      <span class="readout font-mono font-bold tabular-nums leading-none text-white">
        {String(Math.round(tuner.position)).padStart(READOUT_DIGITS, "0")}
      </span>
    </div>

    <div class="flex items-end gap-3 min-w-0">
      <div class="flex flex-col items-end min-w-0">
        <span class="text-[9px] sm:text-[10px] 2xl:text-sm tracking-[0.3em] text-neon-red">
          {tuner.locked ? LOCKED_LABEL : TUNING_LABEL}
        </span>
        <span
          class="max-w-full truncate text-xs sm:text-base 2xl:text-2xl font-bold tracking-wider transition-opacity duration-500 {tuner.locked
            ? 'opacity-100'
            : 'opacity-30'}"
        >
          {tuner.station.name}
        </span>
      </div>
      <div class="eq flex items-end gap-0.5 h-7 sm:h-9 2xl:h-14 short:h-6" class:eq-live={tuner.locked}>
        {#each EQ_BARS as bar (bar)}
          <span class="w-1 2xl:w-1.5 h-full rounded-full bg-neon-red" style="--bar: {bar}"></span>
        {/each}
      </div>
    </div>
  </div>

  <!-- FREQUENCY BAND: the strip slides under a fixed needle -->
  <div class="band relative h-12 sm:h-14 2xl:h-20 short:h-9 overflow-hidden border-y border-white/10">
    <div
      class="strip absolute inset-y-0 left-1/2"
      style="--position: {tuner.position}; --unit: {DIAL_UNIT_PX}px; width: {DIAL_WIDTH_PX}px"
    >
      <div class="ticks ticks-minor absolute inset-x-0 bottom-0 h-1/3 text-white/25"></div>
      <div class="ticks ticks-major absolute inset-x-0 bottom-0 h-3/5 text-white/60"></div>
      {#each DIAL_LABELS as label (label)}
        <span
          class="absolute top-0.5 -translate-x-1/2 font-mono text-[9px] sm:text-[10px] 2xl:text-sm text-muted"
          style="left: calc({label} * var(--unit))"
        >
          {String(label).padStart(READOUT_DIGITS, "0")}
        </span>
      {/each}
    </div>
    <div class="needle absolute inset-y-0 left-1/2 w-px bg-neon-red text-neon-red"></div>
  </div>
</section>

<style>
  /* The readout scales with the dial's own width and with screen height, so it fits beside the station name */
  .readout {
    /* Chrome < 105 and Safari < 16 have no container units and keep this viewport-based size */
    font-size: clamp(2.5rem, 9vw, 6.5rem);
    font-size: clamp(1.75rem, min(18cqw, 12vh), 6.5rem);
  }

  /* Fade the band out at both ends so stations appear to roll in from nowhere */
  .band {
    -webkit-mask-image: linear-gradient(to right, transparent, black 20%, black 80%, transparent);
    mask-image: linear-gradient(to right, transparent, black 20%, black 80%, transparent);
  }

  .strip {
    transform: translate3d(calc(var(--position) * var(--unit) * -1), 0, 0);
    will-change: transform;
  }

  /* Tick marks are drawn in the element's text color, so the color itself stays a Tailwind token */
  .ticks {
    background-image: repeating-linear-gradient(to right, currentColor 0 1px, transparent 1px var(--tick-gap));
  }

  .ticks-minor {
    --tick-gap: calc(var(--unit) * 5);
  }

  .ticks-major {
    --tick-gap: calc(var(--unit) * 25);
  }

  .needle {
    box-shadow: 0 0 14px 1px currentColor;
  }

  /* Equalizer: bars rest low while tuning and bounce once a station is locked */
  .eq span {
    transform-origin: bottom;
    transform: scaleY(0.15);
    transition: transform 400ms ease-out;
  }

  .eq-live span {
    animation: eq-bounce 900ms ease-in-out infinite alternate;
    animation-delay: calc(var(--bar) * -230ms);
  }

  @keyframes eq-bounce {
    from {
      transform: scaleY(0.2);
    }
    to {
      transform: scaleY(1);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .eq-live span {
      animation: none;
      transform: scaleY(0.6);
    }
  }
</style>
