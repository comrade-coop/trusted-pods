// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.13;

import {Test, console2} from "forge-std/Test.sol";
import {PaymentChannel, PaymentChannelFactory} from "../src/PaymentChannelFactory.sol";
import {MockToken} from "../src/MockToken.sol";
import {IERC20Errors} from "openzeppelin-contracts/contracts/interfaces/draft-IERC6093.sol";

contract PaymentTest is Test {
    PaymentChannelFactory public paymentFactory;
    MockToken public token;
    address provider;
    address publisher;

    function setUp() public {
        paymentFactory = new PaymentChannelFactory();
        publisher = vm.createWallet("publisher").addr;
        provider = vm.createWallet("provider").addr;
        token = new MockToken();
    }

    function test_createChannel() public {
        vm.startPrank(publisher);
        token.mint(1000);

        vm.expectRevert();
        paymentFactory.createChannel(provider, token, 1, bytes32(0), 500);

        token.approve(address(paymentFactory), 500);
        PaymentChannel channel = paymentFactory.createChannel(provider, token, 1, bytes32(0), 500);

        assertEq(500, token.balanceOf(address(channel)));

        token.approve(address(paymentFactory), 500);

        vm.expectRevert();
        paymentFactory.createChannel(provider, token, 1, bytes32(0), 500);

        paymentFactory.createChannel(provider, token, 1, bytes32(uint256(1)), 500);
    }

    function test_deposit() public {
        vm.startPrank(publisher);
        token.mint(900);

        token.approve(address(paymentFactory), 500);
        PaymentChannel channel = paymentFactory.createChannel(provider, token, 1, bytes32(0), 500);
        assertEq(token.balanceOf(address(channel)), 500);

        token.approve(address(paymentFactory), 300);
        paymentFactory.deposit(channel, 300);
        assertEq(token.balanceOf(address(channel)), 800);

        token.approve(address(channel), 100);
        channel.deposit(100);
        assertEq(token.balanceOf(address(channel)), 900);

        vm.startPrank(provider);
        token.mint(100);
        token.approve(address(channel), 100);
        channel.deposit(100);
        assertEq(token.balanceOf(address(channel)), 1000);
    }

    function test_withdraw() public {
        vm.startPrank(publisher);
        token.mint(500);
        token.approve(address(paymentFactory), 500);
        PaymentChannel channel = paymentFactory.createChannel(provider, token, 1, bytes32(0), 500);

        vm.startPrank(provider);
        token.mint(100);

        channel.withdraw(25, address(0));
        assertEq(token.balanceOf(provider), 125);
        channel.withdraw(25, address(0));
        assertEq(token.balanceOf(provider), 150);

        vm.expectRevert(PaymentChannel.AmountRequired.selector);
        channel.withdrawUpTo(25, address(0));
        vm.expectRevert(PaymentChannel.AmountRequired.selector);
        channel.withdrawUpTo(50, address(0));

        channel.withdrawUpTo(100, address(0));
        assertEq(token.balanceOf(provider), 200);

        vm.expectRevert(PaymentChannel.InsufficientFunds.selector);
        channel.withdrawUpTo(501, address(1));

        channel.withdrawUpTo(500, address(1));
        assertEq(token.balanceOf(provider), 200);
        assertEq(token.balanceOf(address(1)), 400);
    }

    function test_unlock() public {
        vm.startPrank(publisher);
        token.mint(500);
        token.approve(address(paymentFactory), 500);
        PaymentChannel channel = paymentFactory.createChannel(provider, token, 20, bytes32(0), 500);

        vm.expectRevert(PaymentChannel.ChannelLocked.selector);
        channel.withdrawUnlocked();

        vm.startPrank(provider);
        channel.withdraw(100, address(0));
        assertEq(100, token.balanceOf(provider));
        vm.startPrank(publisher);

        vm.expectRevert(PaymentChannel.ChannelLocked.selector);
        channel.withdrawUnlocked();

        channel.unlock();
        // advance the block timestamp
        vm.warp(block.timestamp + 20);

        channel.withdrawUnlocked();
        assertEq(400, token.balanceOf(publisher));

        vm.expectRevert(PaymentChannel.AmountRequired.selector);
        channel.withdrawUnlocked();
    }

    function test_unlock_withdraw() public {
        vm.startPrank(publisher);
        token.mint(500);
        token.approve(address(paymentFactory), 500);
        PaymentChannel channel = paymentFactory.createChannel(provider, token, 20, bytes32(0), 500);

        channel.unlock();
        vm.warp(10);

        vm.expectRevert(PaymentChannel.ChannelLocked.selector);
        channel.withdrawUnlocked();

        vm.startPrank(provider);
        channel.withdrawUpTo(200, address(0));
        assertEq(token.balanceOf(provider), 200);

        vm.startPrank(publisher);

        channel.withdrawUnlocked();
        assertEq(token.balanceOf(publisher), 300);
    }
}
