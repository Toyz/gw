import "./styles.css";
import "@toyz/loom/element/icon"; // registers <loom-icon>
import "./icons"; // registers our glyph set

import { app } from "@toyz/loom";
import { LoomRouter } from "@toyz/loom/router";

import { RepoService } from "./repo";
import "./app"; // shell
import "./pages/home";
import "./pages/extensions";
import "./pages/ci";

const router = new LoomRouter({ mode: "hash" });
app.use(router);
app.use(new RepoService()); // LoomLifecycle.start() runs on app.start()
app.start();
