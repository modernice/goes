<script lang="ts" setup>
import CardLinks from '../../components/CardLinks.vue'
import CardLink from '../../components/CardLink.vue'
</script>
# Installation

You can setup a goes application manually, ~~or by using the [CLI](/cli/)~~ (TODO).
Use `go get` to download the latest release, and make sure to install nested
modules with `/...`.

```sh
go get github.com/modernice/goes/...
```

### Install a specific commit

goes is still a work-in-progress. You may need to install from a specific commit
to make use of the latest improvements:

```sh
go get github.com/modernice/goes/...@<commit-hash>
```

## Next Steps

<CardLinks>
<CardLink :title="{ as: 'h3', text: 'Events' }" description="Use the event system to communicate between services." link="/guide/events/" />
<CardLink :title="{ as: 'h3', text: 'Aggregates' }" description="Build event-sourced aggregates on top of the event system." link="/guide/events/" />
<CardLink :title="{ as: 'h3', text: 'Projections' }" description="Create arbitrary projections from event streams." link="/guide/events/" />
<CardLink :title="{ as: 'h3', text: 'Commands' }" description="Handle commands between inter-process services." link="/guide/events/" />
<CardLink :title="{ as: 'h3', text: 'Process Managers' }" description="Setup complex inter-service transactions. (soon)" link="/guide/events/" />
<CardLink :title="{ as: 'h3', text: 'Modules' }" description="Integrate pre-built modules into your app." link="/guide/events/" />
</CardLinks>
