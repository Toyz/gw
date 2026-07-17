import "./styles.css";
import "@toyz/loom/element/icon"; // registers <loom-icon>
import "./icons"; // registers our glyph set

import { app } from "@toyz/loom";
import { LoomRouter } from "@toyz/loom/router";

import { RepoService } from "./repo";
import "./win"; // <gw-term> / <gw-code> window components
import "./app"; // shell
import "./pages/home";
import "./pages/recipes";
import "./pages/extensions";
import "./pages/config";
import "./pages/ci";
import "./pages/mcp";

const router = new LoomRouter({ mode: "hash" });
app.use(router);
app.use(new RepoService()); // LoomLifecycle.start() runs on app.start()
app.start();
