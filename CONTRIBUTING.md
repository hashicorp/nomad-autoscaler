# Contributing to Nomad Autoscaler

**First:** if you're unsure or afraid of _anything_, just ask or submit the
issue or pull request anyways. You won't be yelled at for giving it your best
effort. The worst that can happen is that you'll be politely asked to change
something. We appreciate any sort of contributions, and don't want a wall of
rules to get in the way of that.

That said, if you want to ensure that a pull request is likely to be merged,
talk to us! You can find out our thoughts and ensure that your contribution
won't clash or be obviated by the Nomad Autoscaler's normal direction. A great
way to do this is via the [Nomad Discussion Forum][discuss].

## Issues

This section will cover what we're looking for in terms of reporting issues.
By addressing all the points we're looking for, it raises the chances we can
quickly merge or address your contributions.

### Reporting an Issue

* Make sure you test against the latest released version. It is possible
  we already fixed the bug you're experiencing. Even better is if you can test
  against `main` or a [nightly][] build, as bugs are fixed regularly.

* Provide steps to reproduce the issue, and if possible include the expected
  results as well as the actual results. Please provide text, not screen shots.

* If you experienced a panic, please create a [gist](https://gist.github.com)
  of the *entire* generated crash log for us to look at. Double check
  no sensitive items were in the log.

### Issue Lifecycle

1. The issue is reported.

2. The issue is verified and categorized by a Nomad Autoscaler collaborator.
   Categorization is done via tags. For example, bugs are marked as "type/bug".

3. Unless it is critical, the issue may be left for a period of time (sometimes
   many weeks), giving outside contributors -- maybe you!? -- a chance to
   address the issue.

4. The issue is addressed in a pull request or commit. The issue will be
   referenced in the commit message so that the code that fixes it is clearly
   linked.

5. The issue is closed. Sometimes, valid issues will be closed to keep
   the issue tracker clean. The issue is still indexed and available for
   future viewers, or can be re-opened if necessary.

## Pull Requests

Thank you for contributing! Here you'll find information on what to include in
your Pull Request (“PR” for short) to ensure it is reviewed quickly, and
possibly accepted.

Before starting work on a new feature or anything besides a minor bug fix, it
is highly recommended to first initiate a discussion with the Nomad Autoscaler
community (either via a GitHub issue or [HashiCorp Discuss][discuss]). This
will save you from wasting days implementing a feature that could be rejected
in the end.

No pull request template is provided on GitHub. The expected changes are often
already described and validated in an existing issue, that should be
referenced. The Pull Request thread should be mainly used for the code review.

**Tip:** Make it small! A focused PR gives you the best chance of having it
accepted. Then, repeat if you have more to propose!

### Building the Nomad Autoscaler

The Nomad Autoscaler can be easily built for local testing or development using
the `make dev` command from the project root directory. This will output the
compiled binary to `./bin/nomad-autoscaler`. You will need to have the
[Go][go_install] language environment installed and other tooling that can be
installed using the `make tools` command.


### Making Changes to Nomad Autoscaler

The first step to making changes is to [fork][] the Nomad Autoscaler
repository. For more details, please consult GitHub's [official
documentation][gh_fork].

1. Navigate to `$GOPATH/src/github.com/hashicorp/nomad-autoscaler`
2. Rename the existing remote's name: `git remote rename origin upstream`.
3. Add your fork as a remote by running
   `git remote add origin <github url of fork>`. For example:
   `git remote add origin https://github.com/myusername/nomad-autoscaler`.
4. Checkout a feature branch: `git checkout -t -b new-feature`
5. Make changes
6. Push changes to the fork when ready to submit PR:
   `git push -u origin new-feature`

By following these steps you can push to your fork to create a PR, but the code
on disk still lives in the spot where the go cli tools are expecting to find
it.

Using different branches in your local fork will allow you to submit multiple
PRs from the same fork.

We may push minor changes to your fork to avoid unecessary back and forth
requests. We will always notify you if we do, and we will only have access to
branches used to create PRs. You may opt-out by unchecking the checkbox that
reads **Allow edits by maintainers** when opening the PR.

### New Plugins

The Nomad Autoscaler uses the [`go-plugin`][go_plugin] library to implement
many parts of its logic. Please check our [official
documentation][docs_plugins] to learn more.

**NOTE:** The Nomad Autoscaler plugin APIs are still under development and are
subject to changes without notice. We appreciate your contribution, but please
keep this in mind. We expect to achieve API stability soon.

#### New Target Plugins

We are always excited to receive PRs that expand the number of targets
supported by the Nomad Autoscaler. Unfortunetly we are not able to test and
maintain new targets at this point.

But we would love to see your work featured in our [Community][docs_community]
page. If you would like to add your plugin, please submit a PR to the [Nomad
repository][nomad_repo] modifying this [file][nomad_docs_community].

By registering your plugin in our Community page we are also able to track and
notify you of any API changes that might require updates to your codebase.

## Contributor License Agreement

We require that all contributors sign our Contributor License Agreement ("CLA")
before we can accept the contribution.

[Learn more about why HashiCorp requires a CLA and what the CLA includes][cla]

[cla]: https://www.hashicorp.com/cla
[discuss]: https://discuss.hashicorp.com/c/nomad
[docs_community]: https://www.nomadproject.io/docs/autoscaling/plugins/external
[docs_plugins]: https://www.nomadproject.io/docs/autoscaling/plugins
[fork]: https://github.com/hashicorp/nomad-autoscaler/fork
[gh_fork]: https://docs.github.com/en/github/getting-started-with-github/fork-a-repo
[go_install]: https://golang.org/doc/install
[go_plugin]: https://github.com/hashicorp/go-plugin
[nightly]: https://github.com/hashicorp/nomad-autoscaler/releases/tag/nightly
[nomad_docs_community]: https://github.com/hashicorp/nomad/blob/main/website/content/docs/autoscaling/plugins/external/index.mdx
[nomad_repo]: https://github.com/hashicorp/nomad
