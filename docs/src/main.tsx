import "./styles.css";
import "@toyz/loom/element/icon"; // registers <loom-icon>
import "./icons"; // registers our glyph set

import { app } from "@toyz/loom";
import { LoomRouter } from "@toyz/loom/router";

import "./app"; // shell
import "./pages/home";
import "./pages/extensions";

const router = new LoomRouter({ mode: "hash" });
app.use(router);
app.start();
