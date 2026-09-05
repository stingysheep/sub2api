# Affiliate art animation investigation

- Symptom: the dashboard referral illustration appeared to have no breathing animation.
- Root cause: the current in-app browser content viewport was 871 px wide. `DashboardView.vue` wraps the illustration in `hidden lg:flex`, so the entire illustration is `display: none` below 1024 px.
- Evidence: the illustration host computed to `display: none` with a zero-size rectangle. The component still defines `float` (2 px over 6.4 s) and `pulse` (1% over 5.8 s), and reduced-motion was not active.
- Fix: changed the dashboard referral banner and artwork breakpoint from `lg` to `md`, so the illustration is rendered in the 871 px in-app viewport. Added `DashboardAffiliateArt.spec.ts` to preserve the tablet-width behavior.
- Verification: at 871 px the host computed to `display: flex`; over 1.1 seconds the top node moved about 0.51 px and the center scale changed from 1.00505 to 1.0003. The focused regression test and `vue-tsc` both passed.
- Follow-up: after two rounds of feedback that the motion remained too subtle, increased node amplitude to 6 px over 7.2 s and center scale to 4% over 6.6 s, with restrained opacity breathing. The longer cycles preserve smooth per-frame movement; the live page reported node transform around -5.87 px.
- Status: fixed, enhanced, and verified.
