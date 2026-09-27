import "@fontsource/alegreya/400.css";
import "@fontsource/alegreya/700.css";
import "@fontsource/alegreya-sans/400.css";
import "@fontsource/alegreya-sans/700.css";
import "./style.css";
import { mount } from "svelte";
import App from "./App.svelte";

mount(App, { target: document.getElementById("app")! });
