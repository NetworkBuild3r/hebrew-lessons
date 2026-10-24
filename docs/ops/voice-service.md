# Voice service

Recorded 2026-10-23 for INIT-020/SPEC-001. No keys and no voice ids belong on this page.

## Balance

The operator's voice account, shown that day: **30,003 credits remaining of 40,000**.

The app calls the API with model `eleven_v3`. On the website, Eleven v3 costs 1 credit per character (letters, spaces, and punctuation). API calls are discounted against that rate. The estimate below uses 1 credit per character, so it is higher than the API will actually spend.

Source: [How much does it cost to generate using Eleven v3?](https://help.elevenlabs.io/hc/en-us/articles/35869113958801-How-much-does-it-cost-to-generate-using-Eleven-v3) (read 2026-10-23).

## What a month of lessons costs

One learner. A unit introduces at most 8 new words. A dialogue is 2 to 8 lines, and speaker b is a separate clip even when the same words already exist in the female voice. A say step reuses the clip for its Hebrew; it does not add one.

| Piece | Allowance used here |
| --- | --- |
| New words | 8 × 25 characters = 200 |
| Dialogue | 6 lines × 80 characters = 480 |
| One unit | 680 credits at the website rate |
| Steady month (2 units a week, 8 units) | 5,440 |
| Heavy month (4 units a week, 16 units) | 10,880 |

30,003 credits cover a heavy month, and still leave room for a first pass of speaker-b lines on dialogues that were published with only the female voice.

Clips already stored are skipped. Generating the same word again does not spend credits.

## Voices

Speaker a, narration, and every clip with no speaker use the female voice (`ELEVENLABS_VOICE_ID`, the code default). That is the voice for words and say steps. Speaker b uses the male voice (`ELEVENLABS_VOICE_ID_MALE`), and only so the two people in a dialogue stay distinct. There is no code default for the male voice. If it is unset, a speaker-b line fails and is never made in the female voice.

The male voice is in Vault at `kv/apps/production/hebrew`, property `ELEVENLABS_VOICE_ID_MALE`, next to the API key. Both apps read that one property from the deployment secret.
