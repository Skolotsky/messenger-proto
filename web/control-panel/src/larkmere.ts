// A copy of what the fixtures need from demo/content (Larkmere). The real data comes from the demo program.
import type { Person, Situation } from './api';

export const LARKMERE_PEOPLE: Person[] = [
  {
    "id": "maya",
    "name": "Maya Okafor",
    "is_agent": false,
    "line": "Product",
    "status": null
  },
  {
    "id": "anna",
    "name": "Anna Lindqvist",
    "is_agent": false,
    "line": "Founder",
    "status": null
  },
  {
    "id": "marco",
    "name": "Marco Bellandi",
    "is_agent": false,
    "line": "Engineering lead",
    "status": null
  },
  {
    "id": "lena",
    "name": "Lena Fischer",
    "is_agent": false,
    "line": "Design",
    "status": null
  },
  {
    "id": "tomas",
    "name": "Tomás Arrieta",
    "is_agent": false,
    "line": "Backend",
    "status": null
  },
  {
    "id": "priya",
    "name": "Priya Nair",
    "is_agent": false,
    "line": "Marketing",
    "status": null
  },
  {
    "id": "sam",
    "name": "Sam Whitaker",
    "is_agent": false,
    "line": "Support lead",
    "status": null
  },
  {
    "id": "jonas",
    "name": "Jonas Weber",
    "is_agent": false,
    "line": "Sales",
    "status": null
  },
  {
    "id": "dele",
    "name": "Dele Adeyemi",
    "is_agent": false,
    "line": "Data",
    "status": null
  },
  {
    "id": "ines",
    "name": "Inès Laurent",
    "is_agent": false,
    "line": "Frontend",
    "status": null
  },
  {
    "id": "kofi",
    "name": "Kofi Mensah",
    "is_agent": false,
    "line": "Mobile",
    "status": null
  },
  {
    "id": "yuki",
    "name": "Yuki Hayashida",
    "is_agent": false,
    "line": "QA",
    "status": null
  },
  {
    "id": "omar",
    "name": "Omar Haddad",
    "is_agent": false,
    "line": "Finance and ops",
    "status": null
  },
  {
    "id": "ruth",
    "name": "Ruth Mendel",
    "is_agent": false,
    "line": "Customer success",
    "status": null
  },
  {
    "id": "nadia",
    "name": "Nadia Karimi",
    "is_agent": false,
    "line": "Backend",
    "status": null
  },
  {
    "id": "felix",
    "name": "Felix Brandt",
    "is_agent": false,
    "line": "Infrastructure",
    "status": null
  },
  {
    "id": "camila",
    "name": "Camila Duarte",
    "is_agent": false,
    "line": "Frontend",
    "status": null
  },
  {
    "id": "arjun",
    "name": "Arjun Menon",
    "is_agent": false,
    "line": "Backend",
    "status": null
  },
  {
    "id": "hana",
    "name": "Hana Novak",
    "is_agent": false,
    "line": "Mobile",
    "status": null
  },
  {
    "id": "theo",
    "name": "Theo Marsh",
    "is_agent": false,
    "line": "Product design",
    "status": null
  },
  {
    "id": "zoe",
    "name": "Zoe Castellan",
    "is_agent": false,
    "line": "Content design",
    "status": null
  },
  {
    "id": "bram",
    "name": "Bram Dekker",
    "is_agent": false,
    "line": "Infrastructure",
    "status": null
  },
  {
    "id": "leila",
    "name": "Leila Farahani",
    "is_agent": false,
    "line": "QA",
    "status": null
  },
  {
    "id": "owen",
    "name": "Owen Prescott",
    "is_agent": false,
    "line": "Account executive",
    "status": null
  },
  {
    "id": "greta",
    "name": "Greta Holm",
    "is_agent": false,
    "line": "Support",
    "status": null
  },
  {
    "id": "mateo",
    "name": "Mateo Ibarra",
    "is_agent": false,
    "line": "Support",
    "status": null
  },
  {
    "id": "sofia",
    "name": "Sofia Ricci",
    "is_agent": false,
    "line": "Customer success",
    "status": null
  },
  {
    "id": "noor",
    "name": "Noor Siddiqui",
    "is_agent": false,
    "line": "Data analyst",
    "status": null
  },
  {
    "id": "ewan",
    "name": "Ewan Galbraith",
    "is_agent": false,
    "line": "Marketing",
    "status": null
  },
  {
    "id": "paula",
    "name": "Paula Szabo",
    "is_agent": false,
    "line": "People and office",
    "status": null
  },
  {
    "id": "ship",
    "name": "Deployer",
    "is_agent": true,
    "line": "Release agent",
    "status": null
  },
  {
    "id": "triage",
    "name": "Triage",
    "is_agent": true,
    "line": "Support agent",
    "status": null
  },
  {
    "id": "field",
    "name": "Researcher",
    "is_agent": true,
    "line": "Research agent",
    "status": null
  },
  {
    "id": "ledger",
    "name": "Metrics",
    "is_agent": true,
    "line": "Metrics agent",
    "status": null
  },
  {
    "id": "watch",
    "name": "Watcher",
    "is_agent": true,
    "line": "Monitoring agent",
    "status": null
  },
  {
    "id": "scribe",
    "name": "Scribe",
    "is_agent": true,
    "line": "Notes agent",
    "status": null
  }
];

export const LARKMERE_SITUATIONS: Situation[] = [
  {
    "id": "flag-names-you-from-a-person",
    "title": "Anna flags a decision for Maya",
    "outcome": "For Maya the row is new in Focus, stays there as kept after she reads it, and leaves when she answers."
  },
  {
    "id": "flag-names-you-from-an-agent",
    "title": "Deployer stops and waits for Maya's approval",
    "outcome": "For Maya the agent's stop is new in Focus and is released when she approves it in the thread."
  },
  {
    "id": "flag-names-nobody",
    "title": "A flag for the whole channel, answered by someone else",
    "outcome": "For Maya it is an ordinary row in Needs you; Marco's answer changes nothing for her, and it leaves when she reads it."
  },
  {
    "id": "direct-mention",
    "title": "Marco mentions Maya",
    "outcome": "For Maya a direct mention lands in Needs you."
  },
  {
    "id": "mention-added-by-edit",
    "title": "Lena edits her message to bring Maya in",
    "outcome": "Maya sees nothing at first; when the edit names her, it reaches her as new in Needs you."
  },
  {
    "id": "mention-removed-by-edit",
    "title": "Jonas takes his mention of Maya back",
    "outcome": "The row appears in Maya's Needs you and disappears without a trace when the edit removes her name before she has read it."
  },
  {
    "id": "edited-to-another-length",
    "title": "Tomás turns a short question into a long one",
    "outcome": "The row stays where it is in Maya's Needs you, unread, with the new text; nothing moves."
  },
  {
    "id": "deleted-after-reaching-the-feed",
    "title": "Priya deletes a message Maya has read and Lena has not",
    "outcome": "Maya, who read it, keeps a marker in her history; Lena, who never saw it, is left with nothing."
  },
  {
    "id": "pile-of-reactions",
    "title": "The team reacts to Maya's announcement",
    "outcome": "For Maya the reactions are one grouped row in For your information, showing the split by emoji."
  },
  {
    "id": "reaction-taken-back",
    "title": "Sam reacts to Maya's question, then takes it back",
    "outcome": "Sam's reaction lands in Maya's Needs you as an answer; when he takes it back before she has seen it, the row is gone."
  },
  {
    "id": "replies-in-a-thread-you-started",
    "title": "Replies arrive in Maya's own thread",
    "outcome": "For Maya the thread is one row in Needs you, however many replies come in."
  },
  {
    "id": "replies-in-a-thread-you-follow",
    "title": "Replies in a thread Maya chose to follow",
    "outcome": "For Maya the replies land in For your information, because she set the thread to that level."
  },
  {
    "id": "replies-in-a-thread-you-turned-off",
    "title": "Maya turns a noisy thread off",
    "outcome": "Replies no longer reach Maya, until one names her: what names you gets through any dial."
  },
  {
    "id": "incident-burst-at-three-dials",
    "title": "A burst in #incidents, seen at Focus, Default and Muted",
    "outcome": "Maya at Focus gets one row in Focus for the whole burst; Tomás at Default and Priya at Muted get nothing, until a message names them, which reaches both in Needs you."
  },
  {
    "id": "agent-progress-result-and-stop",
    "title": "Researcher works, reports and stops for Maya",
    "outcome": "For Maya the progress and the result stay in For your information on one row; when the agent stops for her, that row rises to Focus."
  },
  {
    "id": "courtesy-mention-info",
    "title": "Priya copies Maya in, marked Info",
    "outcome": "For Maya a mention marked Info lands in For your information and sends no notification."
  }
];
