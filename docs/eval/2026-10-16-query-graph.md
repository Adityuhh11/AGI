# Eval report: query-graph

Date: 2026-10-16. Embedder: `google/embeddinggemma-2`. Golden queries: 92. Indexed: 96 files, 431 chunks (8 denied, 0 errors).

| metric | value |
|---|---|
| top1 | 0.896 |
| recall@3 | 1.000 |
| recall@10 | 1.000 |
| MRR | 0.946 |
| forbidden hits | 0 |
| false alarms (answerable, not confident) | 0.000 |
| missed nones (unanswerable, confident) | 0.000 |

## Per query

| # | query | first relevant rank | expected | top result |
|---|---|---|---|---|
| 1 | find places near the user within a given radius | 1 | locations.controller.js, models/location.js, locations.route.js | NextPlore/Backend/src/controllers/locations.controller.js |
| 2 | hash the user's password before saving it | 1 | models/user.js | NextPlore/Backend/src/models/user.js |
| 3 | email a verification code to a new user | 1 | emailService.js, user.controller.js | NextPlore/Backend/src/controllers/user.controller.js |
| 4 | issue a new access token from a refresh token | 1 | utils/jwt.js, user.controller.js, authService.js | NextPlore/Backend/src/controllers/user.controller.js |
| 5 | map component that shows venue markers | 1 | components/Map.jsx | NextPlore/Frontend/src/components/Map.jsx |
| 6 | many to many relation between locations and tags | 1 | models/association.js, NextPlore/README.md, dataschemas.png | NextPlore/README.md |
| 7 | redirect logged in users away from the login page | 1 | GuestRoute.jsx | NextPlore/Frontend/src/components/GuestRoute.jsx |
| 8 | protect pages so only authenticated users can open them | 1 | ProtectedRoute.jsx | NextPlore/Frontend/src/components/ProtectedRoute.jsx |
| 9 | catch errors thrown while rendering React components | 1 | ErrorBoundary.jsx | NextPlore/Frontend/src/components/ErrorBoundary.jsx |
| 10 | connect to PostgreSQL using Sequelize | 1 | db/database.js, Backend/index.js | NextPlore/Backend/src/db/database.js |
| 11 | wrapper that catches errors in async express route handlers | 1 | asynchandler.js | NextPlore/Backend/src/utils/asynchandler.js |
| 12 | custom error class with an HTTP status code | 1 | apiError.js | NextPlore/Backend/src/utils/apiError.js |
| 13 | load country data from a CSV into the database | 1 | addcountries.js | NextPlore/Backend/addcountries.js |
| 14 | script that pulls foursquare places and creates tags | 1 | updateLocationsAndTags.js, addlocations.js | NextPlore/Backend/updateLocationsAndTags.js |
| 15 | page where a user resets a forgotten password | 1 | ForgotPassword.jsx | NextPlore/Frontend/src/pages/ForgotPassword.jsx |
| 16 | search input that filters venues | 2 | SearchBar.jsx, FilterPage.jsx | NextPlore/Backend/DATA-CSVs/nextplore Data - nextplore.csv |
| 17 | only let users in after their email address was verified | 1 | UnverifiedRoute.jsx, VerifyEmail.jsx | NextPlore/Frontend/src/components/UnverifiedRoute.jsx |
| 18 | addcountries | 1 | addcountries.js | NextPlore/Backend/addcountries.js |
| 19 | ProtectedRoute | 1 | ProtectedRoute.jsx, App.jsx | NextPlore/Frontend/src/components/ProtectedRoute.jsx |
| 20 | emailService | 1 | emailService.js | NextPlore/Backend/src/utils/emailService.js |
| 21 | GEOGRAPHY POINT | 1 | models/location.js, dataschemas.png, NextPlore/README.md | NextPlore/Backend/src/models/location.js |
| 22 | how do I set up the backend environment variables | 1 | NextPlore/README.md | NextPlore/README.md |
| 23 | who built which part of the project | 1 | NextPlore/README.md | NextPlore/README.md |
| 24 | which parts of the project were written with AI assistance | 1 | NextPlore/README.md | NextPlore/README.md |
| 25 | which database index makes the radius search fast | 1 | NextPlore/README.md | NextPlore/README.md |
| 26 | JWT_ACCESS_SECRET | 1 | NextPlore/README.md, utils/jwt.js | NextPlore/Backend/src/utils/jwt.js |
| 27 | how to calculate a confidence interval | 1 | Week 10 Prep.pdf | Week 10 Prep.pdf |
| 28 | standard error of the mean worked example | 1 | Week 10 Prep.pdf | Week 10 Prep.pdf |
| 29 | what changes when the population standard deviation is unknown | 1 | Prep.pdf | Week 10 Prep.pdf |
| 30 | table of critical values for the t distribution | 1 | Week 10 Prep.pdf | Week 10 Prep.pdf |
| 31 | prior distribution times likelihood gives the posterior | 1 | Week 11 Prep.pdf | Week 11 Prep.pdf |
| 32 | bayesian updating of the mean as more data points are seen | 1 | Week 11 Prep.pdf | Week 11 Prep.pdf |
| 33 | entity relationship diagram of the database tables | 2 | dataschemas.png | NextPlore/README.md |
| 34 | which columns does the users table have | 1 | dataschemas.png, models/user.js | NextPlore/Backend/src/models/user.js |
| 35 | n8n workflow for collecting data from the foursquare api | 2 | datapipeline.png | NextPlore/README.md |
| 36 | app logo white text with a red arrow on black | 1 | Logo V1.png | NextPlore/Documentation/Logo/Logo V1.png |
| 37 | man hiking on a mountain at night under the stars | 1 | 07.06.58 | WhatsApp Image 2026-09-16 at 07.06.58.jpeg |
| 38 | football player in a white kit standing in the rain | 1 | cristiano-ronaldo | wp6606097-cristiano-ronaldo-computer-wallpapers.jpg |
| 39 | dark dashboard with a leaderboard of top innovators, XP and badges | 1 | Screenshot 2025-05-21 171057 | Screenshot 2025-05-21 171057.png |
| 40 | user flow steps log in, quests, ask, connect, build on a pink background | 1 | Screenshot 2025-05-21 213138, Screenshot 2025-05-21 213513 | Screenshot 2025-05-21 213138.png |
| 41 | five stick figure sketches joined by black lines | 1 | Screenshot 2025-05-22 104608, Screenshot 2025-05-21 213513 | Screenshot 2025-05-22 104608.png |
| 42 | python function that nudges a weight by epsilon to check the gradient | 1 | Screenshot 2026-07-26 125324 | Screenshot 2026-07-26 125324.png |
| 43 | cinema seat selection for a Spider-Man screening | 1 | Screenshot 2026-07-29 233342 | Screenshot 2026-07-29 233342.png |
| 44 | code editor next to an AI chat summary about a personal assistant with text to speech | 1 | Screenshot 2026-08-17 214116 | Screenshot 2026-08-17 214116.png |
| 45 | red barber shop illustration with a Hindi title and a music player | 1 | Screenshot 2026-09-02 152836, Screenshot 2026-09-02 154825 | Screenshot 2026-09-02 154825.png |
| 46 | shirtless man cooking in a kitchen with pink floral wallpaper | 1 | Screenshot 2026-09-05 225127 | Screenshot 2026-09-05 225127.png |
| 47 | cricket batsman playing a drive in a white kit | 1 | Virat Kholi | Virat Kholi.jpg |
| 48 | hand holding a small blue circuit board with colourful jumper wires | 2 | 18.06.03 | WhatsApp Image 2026-04-14 at 18.06.02.jpeg |
| 49 | close up of Raspberry Pi GPIO pins with jumper cables next to a laptop keyboard | 2 | 18.06.02 | WhatsApp Image 2026-04-14 at 18.06.03 (1).jpeg |
| 50 | man in a mustard shirt standing against a wall with a long shadow | 1 | 20.43.19 | WhatsApp Image 2026-07-16 at 20.43.19 (1).jpeg |
| 51 | jellyfish floating around a snowy mountain, black and white illustration | 1 | 07.06.16 | WhatsApp Image 2026-09-16 at 07.06.16.jpeg |
| 52 | smiling woman in a saree in front of green leaves | 1 | bharathi prem | bharathi prem.jpg |
| 53 | man walking towards a dark mountain with red text saying what could possibly go wrong | 1 | images.genius.com | https___images.genius.com_3a4fb1d9ba535b65dee3bc82d5d666d3.1000x1000x1.png |
| 54 | comic book joker with green hair and a big grin | 1 | joker licking lips | joker licking lips.jpg |
| 55 | ascii art of a hand made out of digits | 1 | right hand | right hand .jpg |
| 56 | snowy mountain range under a dark sky | 1 | sukant-sharma | sukant-sharma-oCGLfr7GSPk-unsplash-bw.jpg |
| 57 | getRecommendLocations | 1 | locations.controller.js, locations.route.js | NextPlore/Backend/src/routes/locations.route.js |
| 58 | request-password-reset | 1 | user.route.js, user.controller.js, authService.js, ForgotPassword.jsx | NextPlore/Backend/src/controllers/user.controller.js |
| 59 | At least one tag is required for recommendations | 1 | locations.controller.js | NextPlore/Backend/src/controllers/locations.controller.js |
| 60 | Passwords do not match | 2 | user.controller.js, ForgotPassword.jsx | NextPlore/Frontend/src/pages/Account.jsx |
| 61 | PG_DATABASE | 1 | NextPlore/README.md, db/database.js | NextPlore/Backend/src/db/database.js |
| 62 | bcryptjs | 1 | Backend/package.json, models/user.js | NextPlore/Backend/src/models/user.js |
| 63 | react-leaflet | 1 | Frontend/package.json, components/Map.jsx | NextPlore/Frontend/src/components/Map.jsx |
| 64 | /refresh-token | 3 | user.route.js | NextPlore/Backend/src/utils/jwt.js |
| 65 | nodemailer | 1 | emailService.js, Backend/package.json | NextPlore/Backend/src/utils/emailService.js |
| 66 | JWT_REFRESH_EXPIRES_IN | 1 | utils/jwt.js, NextPlore/README.md | NextPlore/Backend/src/utils/jwt.js |
| 67 | locationService | 1 | locationService.js | NextPlore/Frontend/src/services/locationService.js |
| 68 | auth.middleware | 1 | auth.middleware.js, user.route.js, locations.route.js, home.route.js | NextPlore/Backend/src/middleware/auth.middleware.js |
| 69 | ForgotPassword.css | 1 | ForgotPassword.css | NextPlore/Frontend/src/styles/ForgotPassword.css |
| 70 | Virat Kholi | 1 | Virat Kholi | Virat Kholi.jpg |
| 71 | cristiano ronaldo wallpaper | 1 | cristiano-ronaldo | wp6606097-cristiano-ronaldo-computer-wallpapers.jpg |
| 72 | bharathi prem | 1 | bharathi prem | bharathi prem.jpg |
| 73 | Screenshot 2026-07-29 233342 | 1 | Screenshot 2026-07-29 233342 | Screenshot 2026-07-29 233342.png |
| 74 | ApiError 404 location not found in getPlaceDetails | 2 | locations.controller.js | NextPlore/Frontend/src/pages/LocationDetails.jsx |
| 75 | nodemailer transporter that sends the verification email | 1 | emailService.js | NextPlore/Backend/src/utils/emailService.js |
| 76 | JWT_ACCESS_EXPIRES_IN how long an access token lives | 1 | utils/jwt.js, NextPlore/README.md | NextPlore/Backend/src/utils/jwt.js |
| 77 | react-leaflet map component with markers | 1 | components/Map.jsx | NextPlore/Frontend/src/components/Map.jsx |
| 78 | recipe for chocolate cake | none (ok) | nothing | NextPlore/Backend/src/controllers/user.controller.js |
| 79 | flight booking confirmation | none (ok) | nothing | NextPlore/Backend/src/controllers/user.controller.js |
| 80 | tax invoice 2021 | none (ok) | nothing | NextPlore/Backend/updateLocationsAndTags.js |
| 81 | how to train a puppy | none (ok) | nothing | Screenshot 2025-05-21 213138.png |
| 82 | symptoms of the flu | none (ok) | nothing | bharathi prem.jpg |
| 83 | lease agreement for the apartment | none (ok) | nothing | Week 10 Prep.pdf |
| 84 | guitar chords for a song | none (ok) | nothing | NextPlore/Backend/src/models/tag.js |
| 85 | bank statement march | none (ok) | nothing | NextPlore/Frontend/src/pages/Account.jsx |
| 86 | yoga routine for beginners | none (ok) | nothing | NextPlore/Backend/src/routes/home.route.js |
| 87 | passport renewal form | none (ok) | nothing | NextPlore/Frontend/src/pages/Account.jsx |
| 88 | kubernetes helm chart values | none (ok) | nothing | NextPlore/Backend/testdb.js |
| 89 | wedding seating plan | none (ok) | nothing | Screenshot 2026-07-29 233342.png |
| 90 | hospital appointment reminder | none (ok) | nothing | NextPlore/Backend/src/utils/emailService.js |
| 91 | sourdough starter feeding schedule | none (ok) | nothing | Week 11 Prep.pdf |
| 92 | visa application checklist for Germany | none (ok) | nothing | NextPlore/Backend/DATA-CSVs/nextplore Data - countries.csv |
