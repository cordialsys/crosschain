This was forked from the upstream Kaspad project.
https://github.com/kaspanet/kaspad/tree/master/domain/consensus/utils/txscript

The upstream `txscript` makes use of a C-linked library for schnorr signatures
which would reduce the portability of crosschain. Fortunately this functionality
is not needed for us and is easy to remove in a fork.
