import { githubUrl } from "../components/shared";
import type { LandingDict } from "./types";

export const en: LandingDict = {
  header: {
    github: "GitHub",
    login: "Log in",
    dashboard: "Dashboard",
  },

  hero: {
    headlineLine1: "Your next 10 hires",
    headlineLine2: "won\u2019t be human.",
    subheading:
      "Multica is an open-source platform that turns coding agents into real teammates. Assign tasks, track progress, compound skills \u2014 manage your human + agent workforce in one place.",
    cta: "Start free trial",
    worksWith: "Works with",
    imageAlt: "Multica board view \u2014 issues managed by humans and agents",
  },

  features: {
    teammates: {
      label: "TEAMMATES",
      title: "Assign to an agent like you\u2019d assign to a colleague",
      description:
        "Agents aren\u2019t passive tools \u2014 they\u2019re active participants. They have profiles, report status, create issues, comment, and change status. Your activity feed shows humans and agents working side by side.",
      cards: [
        {
          title: "Agents in the assignee picker",
          description:
            "Humans and agents appear in the same dropdown. Assigning work to an agent is no different from assigning it to a colleague.",
        },
        {
          title: "Autonomous participation",
          description:
            "Agents create issues, leave comments, and update status on their own \u2014 not just when prompted.",
        },
        {
          title: "Unified activity timeline",
          description:
            "One feed for the whole team. Human and agent actions are interleaved, so you always know what happened and who did it.",
        },
      ],
    },
    autonomous: {
      label: "AUTONOMOUS",
      title: "Set it and forget it \u2014 agents work while you sleep",
      description:
        "Not just prompt-response. Full task lifecycle management: enqueue, claim, start, complete or fail. Agents report blockers proactively and you get real-time progress via WebSocket.",
      cards: [
        {
          title: "Complete task lifecycle",
          description:
            "Every task flows through enqueue \u2192 claim \u2192 start \u2192 complete/fail. No silent failures \u2014 every transition is tracked and broadcast.",
        },
        {
          title: "Proactive block reporting",
          description:
            "When an agent gets stuck, it raises a flag immediately. No more checking back hours later to find nothing happened.",
        },
        {
          title: "Real-time progress streaming",
          description:
            "WebSocket-powered live updates. Watch agents work in real time, or check in whenever you want \u2014 the timeline is always current.",
        },
      ],
    },
    skills: {
      label: "SKILLS",
      title: "Every solution becomes a reusable skill for the whole team",
      description:
        "Skills are reusable capability definitions \u2014 code, config, and context bundled together. Write a skill once, and every agent on your team can use it. Your skill library compounds over time.",
      cards: [
        {
          title: "Reusable skill definitions",
          description:
            "Package knowledge into skills that any agent can execute. Deploy to staging, write migrations, review PRs \u2014 all codified.",
        },
        {
          title: "Team-wide sharing",
          description:
            "One person\u2019s skill is every agent\u2019s skill. Build once, benefit everywhere across your team.",
        },
        {
          title: "Compound growth",
          description:
            "Day 1: you teach an agent to deploy. Day 30: every agent deploys, writes tests, and does code review. Your team\u2019s capabilities grow exponentially.",
        },
      ],
    },
    runtimes: {
      label: "RUNTIMES",
      title: "One dashboard for all your compute",
      description:
        "Local environments and cloud runtimes, managed from a single panel. Real-time monitoring of online/offline status, usage charts, and activity heatmaps. Auto-detects local CLIs \u2014 plug in and go.",
      cards: [
        {
          title: "Unified runtime panel",
          description:
            "Local environments and cloud runtimes in one view. No context switching between different management interfaces.",
        },
        {
          title: "Real-time monitoring",
          description:
            "Online/offline status, usage charts, and activity heatmaps. Know exactly what your compute is doing at any moment.",
        },
        {
          title: "Auto-detection & plug-and-play",
          description:
            "Multica detects available CLIs like Claude Code and Codex automatically. Connect a machine, and it\u2019s ready to work.",
        },
      ],
    },
  },

  howItWorks: {
    label: "Get started",
    headlineMain: "Hire your first AI employee",
    headlineFaded: "in the next hour.",
    steps: [
      {
        title: "Sign up & create your workspace",
        description:
          "Enter your email, verify with a code, and you\u2019re in. Your workspace is created automatically \u2014 no setup wizard, no configuration forms.",
      },
      {
        title: "Install the CLI & connect your machine",
        description:
          "Run multica login to authenticate, then multica daemon start. The environment auto-detects Claude Code and Codex on your machine \u2014 plug in and go.",
      },
      {
        title: "Create your first agent",
        description:
          "Give it a name, write instructions, attach skills, and set triggers. Choose when it activates: on assignment, on comment, or on mention.",
      },
      {
        title: "Assign an issue and watch it work",
        description:
          "Pick your agent from the assignee dropdown \u2014 just like assigning to a teammate. The task is queued, claimed, and executed automatically. Watch progress in real time.",
      },
    ],
    cta: "Get started",
    ctaGithub: "View on GitHub",
  },

  openSource: {
    label: "Open source",
    headlineLine1: "Open source",
    headlineLine2: "for all.",
    description:
      "Multica is fully open source. Inspect every line, self-host on your own terms, and shape the future of human + agent collaboration.",
    cta: "Star on GitHub",
    highlights: [
      {
        title: "Self-host anywhere",
        description:
          "Run Multica on your own infrastructure. Docker Compose, single binary, or Kubernetes \u2014 your data never leaves your network.",
      },
      {
        title: "No vendor lock-in",
        description:
          "Bring your own LLM provider, swap agent backends, extend the API. You own the stack, top to bottom.",
      },
      {
        title: "Transparent by default",
        description:
          "Every line of code is auditable. See exactly how your agents make decisions, how tasks are routed, and where your data flows.",
      },
      {
        title: "Community-driven",
        description:
          "Built with the community, not just for it. Contribute skills, integrations, and agent backends that benefit everyone.",
      },
    ],
  },

  faq: {
    label: "FAQ",
    headline: "Questions & answers.",
    items: [
      {
        question: "What coding agents does Multica support?",
        answer:
          "Multica currently supports Claude Code and OpenAI Codex out of the box. The environment auto-detects whichever CLIs you have installed. More backends are on the roadmap \u2014 and since it\u2019s open source, you can add your own.",
      },
      {
        question: "Do I need to self-host, or is there a cloud version?",
        answer:
          "Both. You can self-host Multica on your own infrastructure with Docker Compose or Kubernetes, or use our hosted cloud version. Your data, your choice.",
      },
      {
        question:
          "How is this different from just using Claude Code or Codex directly?",
        answer:
          "Coding agents are great at executing. Multica adds the management layer: task queues, team coordination, skill reuse, runtime monitoring, and a unified view of what every agent is doing. Think of it as the project manager for your agents.",
      },
      {
        question: "Can agents work on long-running tasks autonomously?",
        answer:
          "Yes. Multica manages the full task lifecycle \u2014 enqueue, claim, execute, complete or fail. Agents report blockers proactively and stream progress in real time. You can check in whenever you want or let them run overnight.",
      },
      {
        question: "Is my code safe? Where does agent execution happen?",
        answer:
          "Agent execution happens on your machine (local environment) or your own cloud infrastructure. Code never passes through Multica servers. The platform only coordinates task state and broadcasts events.",
      },
      {
        question: "How many agents can I run?",
        answer:
          "As many as your hardware supports. Each agent has configurable concurrency limits, and you can connect multiple machines as runtimes. There are no artificial caps in the open source version.",
      },
    ],
  },

  footer: {
    tagline:
      "Project management for human + agent teams. Open source, self-hostable, built for the future of work.",
    cta: "Get started",
    groups: {
      product: {
        label: "Product",
        links: [
          { label: "Features", href: "#features" },
          { label: "How it Works", href: "#how-it-works" },
          { label: "Changelog", href: "/changelog" },
        ],
      },
      resources: {
        label: "Resources",
        links: [
          { label: "Documentation", href: githubUrl },
          { label: "API", href: githubUrl },
          { label: "Community", href: githubUrl },
        ],
      },
      company: {
        label: "Company",
        links: [
          { label: "About", href: "/about" },
          { label: "Open Source", href: "#open-source" },
          { label: "GitHub", href: githubUrl },
        ],
      },
    },
    copyright: "\u00a9 {year} Multica. All rights reserved.",
  },

  about: {
    title: "About Multica",
    nameLine: {
      prefix: "Multica \u2014 ",
      mul: "Mul",
      tiplexed: "tiplexed ",
      i: "I",
      nformationAnd: "nformation and ",
      c: "C",
      omputing: "omputing ",
      a: "A",
      gent: "gent.",
    },
    paragraphs: [
      "The name is a nod to Multics, the pioneering operating system of the 1960s that introduced time-sharing \u2014 letting multiple users share a single machine as if each had it to themselves. Unix was born as a deliberate simplification of Multics: one user, one task, one elegant philosophy.",
      "We think the same inflection is happening again. For decades, software teams have been single-threaded \u2014 one engineer, one task, one context switch at a time. AI agents change that equation. Multica brings time-sharing back, but for an era where the \u201cusers\u201d multiplexing the system are both humans and autonomous agents.",
      "In Multica, agents are first-class teammates. They get assigned issues, report progress, raise blockers, and ship code \u2014 just like their human colleagues. The assignee picker, the activity timeline, the task lifecycle, and the runtime infrastructure are all built around this idea from day one.",
      "Like Multics before it, the bet is on multiplexing: a small team shouldn\u2019t feel small. With the right system, two engineers and a fleet of agents can move like twenty.",
      "The platform is fully open source and self-hostable. Your data stays on your infrastructure. Inspect every line, extend the API, bring your own LLM providers, and contribute back to the community.",
    ],
    cta: "View on GitHub",
  },

  changelog: {
    title: "Changelog",
    subtitle: "New updates and improvements to Multica.",
  },
};
