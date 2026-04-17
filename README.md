# deepwork

Bloqueur de sites web configurable pour macOS. CLI Go, local, versionnable.

Inspiré par [Freedom.to](https://freedom.to) et [OneFocus](https://oneapp.one/onefocus), mais minimaliste, open-source, et scriptable.

## Installation

```sh
curl -fsSL https://raw.githubusercontent.com/sebastien/deepwork/main/install.sh | bash
```

Un seul prompt `sudo` pendant l'installation, zéro ensuite.

## Utilisation

```sh
deepwork edit      # ouvre ~/.deepwork/config.yml dans $EDITOR
deepwork start     # active le scheduling
deepwork status    # état courant
deepwork now 25m   # focus ponctuel de 25 minutes
deepwork doctor    # diagnostique les contournements DoH
deepwork stop      # desactive
```

## Configuration

`~/.deepwork/config.yml` :

```yaml
# timezone: Europe/Paris   # optionnel, par defaut fuseau local

sites:
  - linkedin.com
  - twitter.com
  - reddit.com
  - youtube.com

schedules:
  - name: deep_work_matin
    days: [mon, tue, wed, thu, fri]
    start: "09:00"
    end: "12:00"

  - name: deep_work_aprem
    days: [mon, tue, wed, thu, fri]
    start: "14:00"
    end: "17:00"
```

Regles :
- Chaque site couvre ses variantes courantes (ajout automatique de `www.`)
- Les creneaux peuvent se chevaucher
- Les creneaux doivent rester dans une meme journee (pas de `22:00` → `02:00` en V1)
- Le daemon relit la config toutes les 60s — pas besoin de redemarrer apres edit

## Commandes

| Commande | Effet |
|---|---|
| `deepwork install` | Installation initiale (requiert sudo) |
| `deepwork uninstall` | Desinstallation propre (requiert sudo) |
| `deepwork start` | Active le scheduling automatique |
| `deepwork stop` | Desactive (et vide `/etc/hosts`) |
| `deepwork status` | Etat courant + warnings DoH |
| `deepwork now <duree>` | Focus ponctuel (ex: `25m`, `2h`, `1h30m`) |
| `deepwork edit` | Ouvre la config dans `$EDITOR`, valide a la sauvegarde |
| `deepwork doctor` | Diagnostic complet des navigateurs |
| `deepwork version` | Version |

## Comment ca marche

Deux binaires :
- `deepwork` — CLI utilisateur, lance par launchd toutes les 60s via `deepwork tick`
- `deepwork-apply` — binaire privilegie (root via `NOPASSWD` sudoers), seul a toucher `/etc/hosts`

Chaque tick compare l'etat desire (config + creneau courant + override ponctuel) a l'etat reel (`/etc/hosts`) et reconcilie en shellant `sudo -n deepwork-apply` uniquement si necessaire.

Le blocage utilise `0.0.0.0` en IPv4 et `::` en IPv6 (echec immediat, plutot que timeout avec `127.0.0.1`) et flush le cache DNS macOS apres chaque modification. Les deux familles sont necessaires : macOS prefere IPv6 quand il est disponible, donc un bloc IPv4-seul laisse passer le trafic.

```
# DEEPWORK_START
0.0.0.0 linkedin.com
:: linkedin.com
0.0.0.0 www.linkedin.com
:: www.linkedin.com
...
# DEEPWORK_END
```

## DNS-over-HTTPS et autres contournements

Si un navigateur utilise DoH (DNS over HTTPS), ses requetes court-circuitent le resolver systeme — donc `/etc/hosts`. `deepwork doctor` inspecte Firefox, Chrome, Edge, Brave et Arc, et indique comment desactiver DoH cas par cas.

Notes :
- **Chrome** garde un cache DNS interne. Apres le premier blocage, visite `chrome://net-internals/#dns` pour le flusher, ou relance Chrome.
- **HSTS** peut produire des erreurs moches sur les sites HTTPS deja visites — c'est attendu, pas un bug.
- **Safari** utilise le resolver systeme (pas de DoH), mais maintient son propre cache DNS in-process. `deepwork-apply` tue `com.apple.WebKit.Networking` apres chaque flush pour le contourner.
- **iCloud Private Relay** bypass totalement `/etc/hosts` pour Safari. A desactiver dans `System Settings → Apple ID → iCloud → Private Relay` si tu veux que le blocage marche sur Safari.

## Desinstallation

```sh
sudo deepwork uninstall
```

Supprime les binaires, le LaunchAgent, la regle sudoers, et les entrees DEEPWORK dans `/etc/hosts`. Ta config (`~/.deepwork/`) est preservee.

## Debug

### Check rapide (< 30s)

```sh
deepwork status                                # etat attendu
sudo grep -B1 -A5 DEEPWORK /etc/hosts          # le block est-il present ?
curl -I https://linkedin.com                   # doit echouer immediatement
```

### Debug ladder

**1. Le LaunchAgent tourne-t-il ?**
```sh
launchctl list | grep deepwork                 # doit montrer com.deepwork.scheduler
```
Si absent : `sudo deepwork uninstall && sudo deepwork install`.

**2. Les logs disent quoi ?**
```sh
tail -f ~/.deepwork/logs/deepwork.log
```
Les transitions du tick (`tick: apply (6 domains)`) apparaissent ici.

**3. Force un tick immediat**
```sh
launchctl kickstart -k gui/$UID/com.deepwork.scheduler
# ou en foreground pour voir les erreurs direct :
deepwork tick
```

**4. `deepwork-apply` marche-t-il ?**
```sh
sudo -n /usr/local/bin/deepwork-apply apply linkedin.com
# doit retourner 0 sans prompt — sinon la regle sudoers est cassee
sudo cat /etc/sudoers.d/deepwork
```

**5. La resolution systeme voit-elle le blocage ?**
```sh
dscacheutil -q host -a name linkedin.com       # doit retourner ip_address: 0.0.0.0
curl -sI https://linkedin.com                   # doit echouer immediatement
```
Note : `dig`, `host`, `nslookup` NE consultent PAS `/etc/hosts` — ils parlent direct au DNS et renvoient toujours la vraie IP. Ca ne veut pas dire que le blocage est casse. `curl` et les navigateurs passent par `getaddrinfo(3)` qui respecte `/etc/hosts`.

**6. Force un etat pour isoler le bug**
```sh
deepwork now 2m                                # force active 2 min, ignore la config
deepwork status                                # doit montrer "Active (override)"
```
Si `now 2m` bloque mais pas un creneau : bug dans `internal/schedule`. Si aucun des deux : bug dans `/etc/hosts` ou dans les permissions.

### Failure modes classiques

| Symptome | Cause probable | Verification |
|---|---|---|
| Le site charge quand meme sur Chrome | Cache DNS interne de Chrome | `chrome://net-internals/#dns` → Clear host cache |
| Le site charge sur Firefox | DoH actif | `deepwork doctor` |
| `deepwork tick` dit "config invalid" | YAML casse apres `deepwork edit` | Relire la stderr du tick |
| `sudo -n deepwork-apply` demande un mot de passe | Sudoers pas installe ou username change | Ligne 1 de `/etc/sudoers.d/deepwork` doit matcher `whoami` |
| `/etc/hosts` modifie mais site accessible | HSTS ou DoH dans le navigateur | `dscacheutil -q host -a name linkedin.com` retourne `0.0.0.0` mais navigateur charge → `deepwork doctor` |
| Status dit "Active" mais `curl` passe | Cache DNS pas flushe | `sudo dscacheutil -flushcache && sudo killall -HUP mDNSResponder` |

### Chaine complete en une commande

```sh
deepwork status && \
  echo "--- hosts ---" && sudo grep -A1 DEEPWORK /etc/hosts && \
  echo "--- launchctl ---" && launchctl list | grep deepwork && \
  echo "--- resolve ---" && dscacheutil -q host -a name linkedin.com && \
  echo "--- curl ---" && curl -sI https://linkedin.com 2>&1 | head -2
```

## Developpement

```sh
make build    # produit bin/deepwork + bin/deepwork-apply
make test     # go test -race ./...
make lint     # go vet + gofmt -l
make fmt      # gofmt -w .
```

Dependances : Go 1.22+ uniquement. `goreleaser` pour les releases.

Structure :

```
cmd/
  deepwork/          # CLI utilisateur
  deepwork-apply/    # binaire privilegie (seul a ecrire /etc/hosts)
internal/
  config/            # parsing YAML + validation + timezone
  schedule/          # IsActive + NextTransition
  state/             # calcul de l'etat desire (config + override)
  store/             # persistance runtime.json
  reconcile/         # logique du tick
  hosts/             # injection marker-delimite + flock + ecriture atomique
  dns/               # flush dscacheutil + mDNSResponder
  launchd/           # generation plist + wrappers launchctl
  lifecycle/         # install/uninstall
  doctor/            # probes DoH par navigateur
  paths/             # constantes de chemins
  domain/            # validateur FQDN strict (frontier de securite)
```

## Menace et limitations V1

Deepwork V1 est **non-adversarial** : pas de mot de passe sur `stop`, pas de cooldown, pas de lock-down. C'est un outil de discipline, pas une prison auto-imposee. Si tu veux un mecanisme strict (a la Freedom), attends V1.1.

## Roadmap

| Version | Feature |
|---|---|
| V1.1 | Mot de passe de bypass + cooldown |
| V1.2 | Blocage d'apps macOS |
| V1.3 | Stats d'usage + notarisation Apple |
| V1.4 | Profils multiples |
| V1.5 | Integration Shortcuts.app |
| V2 | Backend `pfctl` (resistant a DoH) |

## License

MIT
