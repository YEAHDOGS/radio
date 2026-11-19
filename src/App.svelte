<script>
  // Import brand image assets as static resource URLs
  import logo from "./assets/dogs-logo-cropped.png";
  import madeByDogs from "./assets/made-by-dogs-small.png";
  import Headphones from "@lucide/svelte/icons/headphones";
  import MicVocal from "@lucide/svelte/icons/mic-vocal";
  import RadioTower from "@lucide/svelte/icons/radio-tower";
  import TvMinimalPlay from "@lucide/svelte/icons/tv-minimal-play";
  import Tuner from "./Tuner.svelte";

  // Constant configurations for layout metrics
  const APP_MAX_WIDTH = "max-w-7xl";
  const CURRENT_YEAR = new Date().getFullYear();
  const WEAREDOGS_URL = "https://dogs.red";

  // Landing copy. The static fallback in index.html mirrors the headline and pitch.
  const STATUS_LABEL = "ON AIR SOON";
  const EYEBROW = "DOGS RADIO · WORK IN PROGRESS";
  const HEADLINE = "What ya listenin to?";
  const PITCH =
    "Hand-picked stations on the air 24/7, in sync for everyone. Tune in with the streaming service you already use, or free on YouTube.";
  const FEATURES = [
    { icon: RadioTower, title: "24/7 stations", text: "Hand-picked playlists that never go off the air." },
    { icon: Headphones, title: "Your service", text: "Plays through the app you already pay for, or free on YouTube." },
    { icon: MicVocal, title: "Live takeovers", text: "When the DJ presses play, the whole station hears it in sync." },
    { icon: TvMinimalPlay, title: "Watch parties", text: "Rooms for watching videos together, with chat built in." },
  ];

  // Shared feature card styles
  const CARD_CLASS =
    "reveal p-3 sm:p-4 short:p-2.5 bg-card/60 hover:bg-card-hover/80 border border-white/5 hover:border-neon-red/30 rounded-xl transition-colors duration-300 group";
  const CARD_TITLE_CLASS =
    "flex items-center gap-2 text-xs sm:text-sm 2xl:text-lg font-bold tracking-wide mb-1 squat:max-md:mb-0 group-hover:text-neon-red transition-colors duration-300";
  const CARD_DESC_CLASS = "text-[10px] sm:text-xs 2xl:text-base text-neutral-500 leading-relaxed squat:max-md:hidden";
</script>

<main
  class="h-dvh max-h-dvh w-full flex flex-col justify-between bg-app text-white p-4 sm:p-6 md:p-8 lg:p-12 short:p-3 select-none overflow-hidden relative"
>
  <!-- Subtle Glowing background grids -->
  <div
    class="absolute inset-0 bg-radial-[ellipse_at_top_right] from-neon-red/8 to-transparent to-50% pointer-events-none"
  ></div>
  <div
    class="absolute inset-0 bg-radial-[ellipse_at_bottom_left] from-neon-red/3 to-transparent to-50% pointer-events-none"
  ></div>

  <!-- TOP NAV BAR -->
  <header
    class="w-full mx-auto {APP_MAX_WIDTH} flex items-center justify-between border-b border-white/5 pb-3 sm:pb-4 short:pb-2 z-10"
  >
    <div class="flex items-center gap-2.5 sm:gap-3">
      <img
        src={logo}
        alt="DOGS Logo"
        class="w-8 h-8 sm:w-10 sm:h-10 2xl:w-14 2xl:h-14 short:w-6 short:h-6 object-contain transition-transform duration-500 hover:rotate-12"
      />
      <h1
        class="text-lg sm:text-xl md:text-2xl short:text-base font-bold tracking-wider text-white"
      >
        DOGS
      </h1>
    </div>

    <p
      class="flex items-center gap-2 rounded-full border border-neon-red/30 bg-neon-red/5 px-3 py-1 short:py-0.5 text-[10px] sm:text-xs 2xl:text-base font-bold tracking-[0.2em] text-neon-red"
    >
      <span class="relative flex size-2">
        <span class="absolute inline-flex size-full rounded-full bg-neon-red opacity-75 motion-safe:animate-ping"></span>
        <span class="relative inline-flex size-2 rounded-full bg-neon-red"></span>
      </span>
      {STATUS_LABEL}
    </p>
  </header>

  <!-- CENTER BODY (Fits strictly inside viewport bounds) -->
  <div
    class="w-full mx-auto {APP_MAX_WIDTH} flex-1 min-h-0 flex flex-col justify-center gap-6 sm:gap-8 lg:gap-10 short:gap-2 my-4 short:my-1 overflow-hidden z-10"
  >
    <!-- HERO: one centered column on portrait screens, pitch beside the dial on landscape and squat ones -->
    <div class="grid grid-cols-1 wide:sm:grid-cols-2 items-center gap-6 sm:gap-8 lg:gap-10 short:gap-4">
      <div
        class="@container flex flex-col items-center wide:sm:items-start gap-3 sm:gap-4 short:gap-1.5 text-center wide:sm:text-left"
      >
        <p class="reveal text-[9px] sm:text-xs 2xl:text-base font-bold tracking-[0.3em] text-neon-red" style="--step: 0">
          {EYEBROW}
        </p>
        <h2 class="headline reveal font-black tracking-tight leading-[0.95] text-balance text-white" style="--step: 1">
          {HEADLINE}
        </h2>
        <p
          class="reveal max-w-xl 2xl:max-w-3xl text-xs sm:text-sm md:text-base 2xl:text-xl text-neutral-400 leading-relaxed short:hidden"
          style="--step: 2"
        >
          {PITCH}
        </p>
      </div>

      <div class="reveal w-full max-w-3xl 2xl:max-w-5xl mx-auto" style="--step: 3">
        <Tuner />
      </div>
    </div>

    <!-- FEATURES CARD GRID SECTION -->
    <!-- Hidden where there is no height for them: short screens narrower than a desktop. On squat phones and
         small tablets the cards shrink to their titles. -->
    <ul class="grid grid-cols-2 md:grid-cols-4 gap-2 sm:gap-4 lg:gap-6 short:gap-3 short:max-lg:hidden">
      {#each FEATURES as feature, index (feature.title)}
        <li class={CARD_CLASS} style="--step: {index + 4}">
          <h3 class={CARD_TITLE_CLASS}>
            <feature.icon class="size-4 2xl:size-6 shrink-0 text-neon-red" aria-hidden="true" />
            {feature.title}
          </h3>
          <p class={CARD_DESC_CLASS}>{feature.text}</p>
        </li>
      {/each}
    </ul>
  </div>

  <!-- BOTTOM FOOTER -->
  <footer
    class="w-full mx-auto {APP_MAX_WIDTH} pt-3 short:pt-2 border-t border-white/5 z-10"
  >
    <div
      class="flex items-center justify-between gap-3 text-[10px] 2xl:text-sm tracking-wider text-neutral-600 select-none"
    >
      <span>&copy; {CURRENT_YEAR} DOGS.</span>
      <a
        href={WEAREDOGS_URL}
        target="_blank"
        rel="noopener noreferrer"
        class="shrink-0 opacity-70 hover:opacity-100 focus-visible:opacity-100 transition-opacity duration-300"
      >
        <img
          src={madeByDogs}
          alt="This product was made by DOGS"
          width="480"
          height="159"
          class="h-6 sm:h-7 lg:h-8 2xl:h-12 short:h-5 w-auto"
        />
      </a>
    </div>
  </footer>
</main>

<style>
  /* h-dvh needs Chrome 108+; older app-tier browsers (see APP_CSS_TARGET) fall back to vh to still fill the screen */
  @supports not (height: 100dvh) {
    main {
      height: 100vh;
      max-height: 100vh;
    }
  }

  /* The headline scales with its own column and with screen height, so it fills the space on any screen
     without pushing the dial out of view. Past one line it wraps to two balanced lines. */
  .headline {
    /* Chrome < 105 and Safari < 16 have no container units and keep this viewport-based size */
    font-size: clamp(1.5rem, 8vw, 6rem);
    font-size: clamp(1.25rem, min(18cqw, 9vh), 8rem);
  }

  /* Staggered entrance: each block rises into place in --step order. Space is reserved, so nothing reflows. */
  .reveal {
    animation: reveal-rise 700ms cubic-bezier(0.16, 1, 0.3, 1) both;
    animation-delay: calc(var(--step, 0) * 90ms);
  }

  @keyframes reveal-rise {
    from {
      opacity: 0;
      transform: translateY(12px);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .reveal {
      animation: none;
    }
  }
</style>
